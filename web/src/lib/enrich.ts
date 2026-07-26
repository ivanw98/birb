import type { components } from "@/api/schema";
import type { LocalSighting } from "@/types";
import { bumpClientUpdatedAt } from "./time";
import { db } from "@/db/db";
import { fromWire } from "@/sync/syncEngine";
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
  | { outcome: "error"; message: string };

// A photo can be in one of two places, and removing it differs completely.
export type PhotoTarget =
  | { kind: "queued"; id: number } // in db.photos only, never uploaded
  | { kind: "attached"; path: string }; // a path on the server row

export type RemovePhotoResult =
  | { outcome: "removed" }
  | { outcome: "conflict" }
  | { outcome: "offline" }
  | { outcome: "error"; message: string };

// Removing a QUEUED photo is a local delete: nothing on the server knows it
// exists, so this works offline and can't conflict.
//
// Removing an ATTACHED photo needs the network. photoPaths can only change via
// PUT — toWire() deliberately omits photoPaths because the batch endpoint
// rejects them — so a removal written locally and marked pending would never
// reach the server. Rather than lose it silently, say we can't do it yet.
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
    return { outcome: "error", message: `remove failed: ${response.status}` };
  } catch {
    // navigator.onLine lied — we never reached the server, so nothing changed.
    return { outcome: "offline" };
  }
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
    const body = { clientUpdatedAt, ...content, photoPaths: row.photoPaths };
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

    return { outcome: "error", message: `save failed: ${response.status}` };
  } catch {
    // navigator.onLine lied - we never reached the server.
    // Fallback to offline path.
    return saveLocally();
  }
}
