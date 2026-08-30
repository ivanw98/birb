import type { components } from "@/api/schema";
import type { LocalSighting } from "@/types";
import { bumpClientUpdatedAt } from "./time";
import { db } from "@/db/db";
import { fromWire, syncNow, syncSettled } from "@/sync/syncEngine";
import { apiClient } from "@/api/client";

type Sighting = components["schemas"]["Sighting"];
type StaleUpdate = components["schemas"]["StaleUpdate"];

// PUT is a full replace, so the form submits every content field.
export interface SightingContent {
  birdId?: string;
  quickNote?: string;
  notes?: string;
}

export type EnrichResult =
  | { outcome: "saved-offline" }
  | { outcome: "saved-online" }
  | { outcome: "conflict"; current: Sighting }
  | { outcome: "gone" } // deleted on another device
  | { outcome: "error"; message: string };

// A photo can be in one of two places, and removing it differs completely.
export type PhotoTarget =
  | { kind: "queued"; id: number } // in db.photos only, never uploaded
  | { kind: "attached"; path: string }; // a path on the server row

export type RemovePhotoResult =
  | { outcome: "removed" }
  | { outcome: "conflict" }
  | { outcome: "offline" }
  | { outcome: "gone" } // deleted on another device
  | { outcome: "error"; message: string };

// A recording can be in one of two places, same shape as PhotoTarget.
export type RecordingTarget =
  { kind: "queued"; id: number } | { kind: "attached"; path: string };

// Queued photos are deleted locally. Attached photos require an online PUT request
// because photo modifications are not supported in offline batch syncs.
export async function removePhoto(
  row: LocalSighting,
  target: PhotoTarget,
): Promise<RemovePhotoResult> {
  if (target.kind === "queued") {
    await db.photos.delete(target.id);
    return { outcome: "removed" };
  }

  if (!navigator.onLine || row.syncStatus !== "synced") {
    return { outcome: "offline" };
  }

  const clientUpdatedAt = bumpClientUpdatedAt(row.clientUpdatedAt);
  const photoPaths = row.photoPaths.filter((p) => p !== target.path);

  try {
    const { data, error, response } = await apiClient.PUT(
      "/api/sightings/{id}",
      {
        params: { path: { id: row.id } },
        // Content fields come from `row`, NOT the form: removing a photo must
        // not commit text the user hasn't pressed Save on yet.
        body: {
          clientUpdatedAt,
          birdId: row.birdId,
          quickNote: row.quickNote,
          notes: row.notes,
          photoPaths,
          recordingPaths: row.recordingPaths,
        },
      },
    );

    if (data) {
      await db.sightings.put(fromWire(data));
      await deleteLocalPhotoForPath(target.path);
      return { outcome: "removed" };
    }
    if (response.status === 409) {
      const stale = error as StaleUpdate;
      await db.sightings.put(fromWire(stale.current));
      return { outcome: "conflict" };
    }
    if (response.status === 404) {
      await adoptRemoteDeletion(row.id);
      return { outcome: "gone" };
    }
    return { outcome: "error", message: `remove failed: ${response.status}` };
  } catch {
    // navigator.onLine lied: we never reached the server, so nothing changed.
    return { outcome: "offline" };
  }
}

export async function removeRecording(
  row: LocalSighting,
  target: RecordingTarget,
): Promise<RemovePhotoResult> {
  if (target.kind === "queued") {
    await db.recordings.delete(target.id);
    return { outcome: "removed" };
  }

  if (!navigator.onLine || row.syncStatus !== "synced") {
    return { outcome: "offline" };
  }

  const clientUpdatedAt = bumpClientUpdatedAt(row.clientUpdatedAt);
  const recordingPaths = row.recordingPaths.filter((p) => p !== target.path);

  try {
    const { data, error, response } = await apiClient.PUT(
      "/api/sightings/{id}",
      {
        params: { path: { id: row.id } },
        body: {
          clientUpdatedAt,
          birdId: row.birdId,
          quickNote: row.quickNote,
          notes: row.notes,
          photoPaths: row.photoPaths,
          recordingPaths,
        },
      },
    );

    if (data) {
      await db.sightings.put(fromWire(data));
      await deleteLocalRecordingForPath(target.path);
      return { outcome: "removed" };
    }
    if (response.status === 409) {
      const stale = error as StaleUpdate;
      await db.sightings.put(fromWire(stale.current));
      return { outcome: "conflict" };
    }
    if (response.status === 404) {
      await adoptRemoteDeletion(row.id);
      return { outcome: "gone" };
    }
    return { outcome: "error", message: `remove failed: ${response.status}` };
  } catch {
    return { outcome: "offline" };
  }
}

// A 404 for a row we hold as synced means it was tombstoned on another device
// (PUT filters tombstones). Adopt that locally (the next pull would conclude
// the same), so the row leaves the list immediately and Undo can restore it.
async function adoptRemoteDeletion(id: string): Promise<void> {
  await db.sightings.update(id, {
    deleted: 1,
    syncStatus: "synced",
    syncError: undefined,
  });
}

// Waits out any in-flight sync pass first (its PUT would overwrite the
// tombstone), then get+update run in one transaction so a concurrent hard-delete can't race it.
export async function deleteSighting(id: string): Promise<boolean> {
  await syncSettled();
  const applied = await db.transaction("rw", db.sightings, async () => {
    const row = await db.sightings.get(id);
    if (!row || row.deleted) return false;
    await db.sightings.update(id, {
      deleted: 1,
      clientUpdatedAt: bumpClientUpdatedAt(row.clientUpdatedAt),
      syncStatus: "pending",
      syncError: undefined,
    });
    return true;
  });
  if (applied) void syncNow();
  return applied;
}

// Un-deletes with a fresh clientUpdatedAt, so the resurrect wins last-write-wins
// over the tombstone on every device. Works for remote deletions too.
export async function undoDelete(id: string): Promise<boolean> {
  const applied = await db.transaction("rw", db.sightings, async () => {
    const row = await db.sightings.get(id);
    if (!row?.deleted) return false;
    await db.sightings.update(id, {
      deleted: 0,
      clientUpdatedAt: bumpClientUpdatedAt(row.clientUpdatedAt),
      syncStatus: "pending",
      syncError: undefined,
    });
    return true;
  });
  if (applied) void syncNow();
  return applied;
}

// Reclaims the ~200KB local cache copy. The uploaded bytes stay in Storage as
// an orphan, the same accepted state exercise 8.4 traces.
async function deleteLocalPhotoForPath(path: string): Promise<void> {
  const [, sightingId, fileName] = path.split("/");
  if (!sightingId || !fileName) return;

  await db.photos
    .where("sightingId")
    .equals(sightingId)
    .and((p) => p.fileName === fileName)
    .delete();
}

async function deleteLocalRecordingForPath(path: string): Promise<void> {
  const [, sightingId, fileName] = path.split("/");
  if (!sightingId || !fileName) return;

  await db.recordings
    .where("sightingId")
    .equals(sightingId)
    .and((r) => r.fileName === fileName)
    .delete();
}
// One save path for the enrichment form.
export async function saveEnrichment(
  row: LocalSighting,
  content: SightingContent,
): Promise<EnrichResult> {
  const clientUpdatedAt = bumpClientUpdatedAt(row.clientUpdatedAt);

  const saveLocally = async (): Promise<EnrichResult> => {
    await db.sightings.update(row.id, {
      birdId: content.birdId,
      quickNote: content.quickNote,
      notes: content.notes,
      clientUpdatedAt,
      syncStatus: "pending",
      syncError: undefined,
    });
    return { outcome: "saved-offline" };
  };

  if (!navigator.onLine || row.syncStatus !== "synced") {
    return saveLocally();
  }

  try {
    const body = {
      clientUpdatedAt,
      ...content,
      photoPaths: row.photoPaths,
      recordingPaths: row.recordingPaths,
    };
    const { data, error, response } = await apiClient.PUT(
      "/api/sightings/{id}",
      {
        params: { path: { id: row.id } },
        body: body,
      },
    );

    if (data) {
      await db.sightings.put(fromWire(data));

      return { outcome: "saved-online" };
    }
    if (response.status === 409) {
      const stale = error as StaleUpdate;
      await db.sightings.put(fromWire(stale.current));

      return { outcome: "conflict", current: stale.current };
    }
    if (response.status === 404) {
      await adoptRemoteDeletion(row.id);
      return { outcome: "gone" };
    }

    return { outcome: "error", message: `save failed: ${response.status}` };
  } catch {
    // navigator.onLine lied - we never reached the server.
    // Fallback to offline path.
    return saveLocally();
  }
}
