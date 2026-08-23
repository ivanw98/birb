import type { LocalSighting } from "@/types";
import type { components } from "../api/schema";
import { db } from "@/db/db";
import { apiClient } from "@/api/client";
import { refreshFeed } from "@/api/feed";
import { getAccessToken } from "@/auth/tokenProvider";
import { photoStore } from "@/photos";
import { bumpClientUpdatedAt } from "@/lib/time";

type SightingSync = components["schemas"]["SightingSync"];
type BatchItemResult = components["schemas"]["BatchItemResult"];
type Sighting = components["schemas"]["Sighting"];
type StaleUpdate = components["schemas"]["StaleUpdate"];

// contract max - bigger batches are rejected
const BATCH_LIMIT = 100;

// ---------- engine state (a tiny external store React can subscribe to) ----
//
export type SyncResult =
  | { ok: true; pushed: number; failed: number }
  | { ok: false; reason: "offline" | "no-auth" | "network" | "busy" };

export interface SyncEngineState {
  syncing: boolean;
  lastResult: SyncResult | null;
}

let state: SyncEngineState = { syncing: false, lastResult: null };
const listeners = new Set<() => void>();

// React hooks can subscribe to re-reender whenever state changes via setState()
function setState(patch: Partial<SyncEngineState>): void {
  state = { ...state, ...patch };
  for (const l of listeners) l();
}

// lightweight event emitter
export function subscribeSync(cb: () => void): () => void {
  listeners.add(cb);
  return () => {
    listeners.delete(cb);
  };
}

export function getSyncState(): SyncEngineState {
  return state;
}

// ---------- wire projections ----------------------------------------------

function toWire(row: LocalSighting): SightingSync {
  return {
    id: row.id,
    observedAt: row.observedAt,
    observedAtOffsetMinutes: row.observedAtOffsetMinutes,
    clientUpdatedAt: row.clientUpdatedAt,
    birdId: row.birdId,
    quickNote: row.quickNote,
    notes: row.notes,
    latitude: row.latitude,
    longitude: row.longitude,
    accuracyM: row.accuracyM,
    deleted: row.deleted ? true : undefined,
    // No photoPaths — the batch endpoint doesn't accept them (photos are attached via PUT after upload)
  };
}

export function fromWire(remote: Sighting): LocalSighting {
  return {
    id: remote.id,
    observedAt: remote.observedAt,
    observedAtOffsetMinutes: remote.observedAtOffsetMinutes,
    clientUpdatedAt: remote.clientUpdatedAt,
    birdId: remote.birdId,
    quickNote: remote.quickNote,
    notes: remote.notes,
    latitude: remote.latitude,
    longitude: remote.longitude,
    accuracyM: remote.accuracyM,
    photoPaths: remote.photoPaths,
    deleted: remote.deleted ? 1 : 0,
    // fromWire always sets syncStatus: "synced" – anything from the server doesn't need pushing.
    syncStatus: "synced",
  };
}

// ---------- reconciliation - handles potential race conditions between user edits and network calls.

// What to do locally with one batch item's result.
export type ReconcileAction =
  { kind: "patch"; patch: Partial<LocalSighting> } | { kind: "purge" };

// Decide the new local state for one batch item.
export function reconcileItem(
  sent: LocalSighting,
  current: LocalSighting | undefined,
  result: BatchItemResult,
): ReconcileAction | null {
  // If the user edits a record while the batch HTTP request is traveling over the wire, clientUpdatedAt changes locally.
  // reconcileItem detects this mismatch and ignores the server response for that record - record stays pending.
  if (!current || current.clientUpdatedAt !== sent.clientUpdatedAt) return null;

  if (result.status === "invalid") {
    // Honor the delete intent locally instead.
    if (sent.deleted) return { kind: "purge" };
    // Otherwise, the server rejects a row (status: "invalid"); its local status is updated to "failed" with the server's error code.
    return {
      kind: "patch",
      patch: {
        syncStatus: "failed",
        syncError: result.error?.code ?? "invalid",
      },
    };
  }

  return {
    kind: "patch",
    patch: { syncStatus: "synced", syncError: undefined },
  };
}

// ---------- push ------------------------------------------------------------

async function pushPending(): Promise<{ pushed: number; failed: number }> {
  let pushed = 0;
  let failed = 0;
  for (;;) {
    const batch = await db.sightings
      .where("syncStatus")
      .equals("pending")
      .limit(BATCH_LIMIT)
      .toArray();

    if (batch.length === 0) break;
    const body = { sightings: batch.map(toWire) };
    const { data, response } = await apiClient.POST("/api/sightings/batch", {
      body: body,
    });

    if (!data) {
      throw new Error(`batch sync failed: ${response.status}`);
    }

    const sentByID = new Map(batch.map((row) => [row.id, row]));

    // runs an atomic local database update for every item returned
    await db.transaction("rw", db.sightings, db.photos, async () => {
      for (const result of data.results) {
        const sent = sentByID.get(result.id);
        if (!sent) continue;

        const current = await db.sightings.get(result.id);
        const action = reconcileItem(sent, current, result);
        if (!action) continue;

        if (action.kind === "purge") {
          await hardDeleteLocal(result.id);
          continue;
        }

        if (action.patch.syncStatus === "failed") failed++;
        else pushed++;

        await db.sightings.update(result.id, action.patch);
      }
    });
  }
  return { pushed, failed };
}

// ---------- pull ------------------------------------------------------------

// Fetches server records to update the local database (for initial setup or multi-device sync).
// Local pending/failed rows win until pushed. The deleted branch runs FIRST:
// a replayed tombstone must never re-enter through the missing-row insert.
async function pullFromServer(): Promise<void> {
  const seen = new Set<string>();
  let cursor: string | undefined;
  do {
    const { data, response } = await apiClient.GET("/api/sightings", {
      params: { query: { limit: 100, cursor, includeDeleted: true } },
    });

    if (!data) throw new Error(`pull failed: ${response.status}`);

    await db.transaction("rw", db.sightings, db.photos, async () => {
      for (const remoteSighting of data.items) {
        seen.add(remoteSighting.id);
        const local = await db.sightings.get(remoteSighting.id);

        if (remoteSighting.deleted) {
          await applyRemoteTombstone(remoteSighting, local);
        } else if (!local) {
          // something exists on the server, but not our local instance!
          await db.sightings.put(fromWire(remoteSighting));
        } else if (
          local.syncStatus === "synced" &&
          Date.parse(remoteSighting.clientUpdatedAt) >
            Date.parse(local.clientUpdatedAt)
        ) {
          // server contains new information!
          await db.sightings.put(fromWire(remoteSighting));
        }
      }
    });

    // explicit null = last page
    cursor = data.nextCursor ?? undefined;
  } while (cursor);

  await removeRowsGoneFromServer(seen);
}

// The deleted branch of the pull merge:
// - no local copy → nothing to remove (and never insert a tombstone)
// - local already deleted → keep it: it backs the Undo banner until GC'd
// - local synced (live) → deleted on another device; remove row + queued photos
// - local pending/failed → LWW: a newer tombstone wins, otherwise the local
//   edit stays and resurrects the sighting on push
async function applyRemoteTombstone(
  remote: Sighting,
  local: LocalSighting | undefined,
): Promise<void> {
  if (!local || local.deleted) return;
  if (
    local.syncStatus === "synced" ||
    Date.parse(remote.clientUpdatedAt) > Date.parse(local.clientUpdatedAt)
  ) {
    await hardDeleteLocal(local.id);
  }
}

async function hardDeleteLocal(id: string): Promise<void> {
  await db.sightings.delete(id);
  await db.photos.where("sightingId").equals(id).delete();
}

// A completed walk is a full snapshot of the account, tombstones included, so
// any local synced row the walk never returned no longer exists on the server
// for this account (fresh sign-in over an old cache, or a locally GC'd
// tombstone). Pending/failed rows are kept — they carry unpushed work.
async function removeRowsGoneFromServer(seen: Set<string>): Promise<void> {
  await db.transaction("rw", db.sightings, db.photos, async () => {
    const synced = await db.sightings
      .where("syncStatus")
      .equals("synced")
      .toArray();
    for (const row of synced) {
      if (!seen.has(row.id)) await hardDeleteLocal(row.id);
    }
  });
}

// A synced tombstone has done its job (the server knows about the delete) and
// exists locally only to back the Undo banner, which cannot outlive a page
// load. Swept at app start so deleted rows don't accumulate forever.
export async function gcSyncedTombstones(): Promise<void> {
  await db.transaction("rw", db.sightings, db.photos, async () => {
    const synced = await db.sightings
      .where("syncStatus")
      .equals("synced")
      .toArray();
    for (const row of synced) {
      if (row.deleted) await hardDeleteLocal(row.id);
    }
  });
}

// ---------- entry point ------------------------------------------------------

// The currently running pass, so a caller who must not race it can wait.
let inFlight: Promise<SyncResult> | null = null;

// Prevents concurrent execution using state.syncing.
// Sequentially runs Push (upload local work) then Pull (download remote updates).
export async function syncNow(): Promise<SyncResult> {
  if (state.syncing) return { ok: false, reason: "busy" };

  const pass = runSyncPass();
  inFlight = pass;
  try {
    return await pass;
  } finally {
    inFlight = null;
  }
}

// Resolves once any in-flight pass has finished, immediately when none is
// running. syncPhotos() issues a PUT that bumps clientUpdatedAt, so anything
// else about to PUT the same sighting has to wait this out or it 409s itself.
// Never rejects: a failed pass still means "no longer in flight".
export function syncSettled(): Promise<void> {
  return inFlight ? inFlight.then(ignore, ignore) : Promise.resolve();
}

function ignore(): void {}

async function runSyncPass(): Promise<SyncResult> {
  if (!navigator.onLine) return finish({ ok: false, reason: "offline" });
  setState({ syncing: true });
  try {
    const token = await getAccessToken();
    if (!token) return finish({ ok: false, reason: "no-auth" });

    const { pushed, failed } = await pushPending();
    await syncPhotos();
    await pullFromServer();

    try {
      await refreshFeed();
    } catch {
      // A stale feed is the feed pane's banner to report, not SyncResult's:
      // the outer catch would claim queued work failed to sync.
    }

    return finish({ ok: true, pushed, failed });
  } catch {
    // Network or API failure: rows stay pending; the next trigger retries.
    return finish({ ok: false, reason: "network" });
  } finally {
    // Ensure syncing is always unlocked.
    setState({ syncing: false });
  }
}

function finish(result: SyncResult): SyncResult {
  setState({ lastResult: result });
  return result;
}

// ---------- photos -----------------------------------------------------------

async function syncPhotos(): Promise<void> {
  const q = await db.photos.where("uploaded").equals(0).toArray();
  if (q.length === 0) return;

  const bySighting = Map.groupBy(q, (p) => p.sightingId);
  for (const [sightingId, photos] of bySighting) {
    const row = await db.sightings.get(sightingId);
    // parent sighting record doesn't exist in the local database yet
    // (or hasn't finished syncing from the server). Deleted rows keep their
    // queued blobs unuploaded in case of Undo.
    if (!row || row.syncStatus !== "synced" || row.deleted) continue;

    const newPaths: string[] = [];
    for (const p of photos) {
      const path = await photoStore.upload(sightingId, p.fileName, p.blob);
      await db.photos.update(p.id!, { uploaded: 1 });
      newPaths.push(path);
    }

    await attachPhotoPaths(row, newPaths);
  }
}

// PUT the full desired content state with the new paths merged in.
async function attachPhotoPaths(
  row: LocalSighting,
  newPaths: string[],
  retry = true,
): Promise<void> {
  const desiredSet = new Set([...row.photoPaths, ...newPaths]);
  // Contract: openapi/openapi.yaml — maxItems: 10 on both Sighting.photoPaths and the PUT body
  const desiredSlice = [...desiredSet].slice(0, 10);

  const body = {
    clientUpdatedAt: bumpClientUpdatedAt(row.clientUpdatedAt),
    birdId: row.birdId,
    quickNote: row.quickNote,
    notes: row.notes,
    photoPaths: desiredSlice,
  };
  const { data, error, response } = await apiClient.PUT("/api/sightings/{id}", {
    params: { path: { id: row.id } },
    body: body,
  });

  if (data) {
    await db.sightings.put(fromWire(data));
    return;
  }

  if (response.status === 409 && retry) {
    const stale = error as StaleUpdate;
    const adopted = fromWire(stale.current);
    await db.sightings.put(adopted);
    // grab the conflicted state (old) to attach photos to
    await attachPhotoPaths(adopted, newPaths, false);
    return;
  }

  if (response.status === 404) {
    // Tombstoned on another device (PUT filters tombstones). Drop the queue
    // for it rather than failing the whole pass; the pull confirms the delete.
    await db.photos.where("sightingId").equals(row.id).delete();
    return;
  }

  throw new Error(`attaching photo paths failed: ${response.status}`);
}
