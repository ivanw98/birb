import { useState } from "react";
import { Badge } from "@/components/ui/badge";
import { formatObservedAt } from "../lib/time";
import type { LocalSighting, SyncStatus } from "../types";
import { PhotoThumb } from "./PhotoThumb";
import { RecordingPlayer } from "./RecordingPlayer";

const SYNC_LABEL: Record<SyncStatus, string> = {
  pending: "Pending sync",
  synced: "Synced",
  failed: "Sync failed",
};

const SYNC_CLASS: Record<SyncStatus, string> = {
  pending: "border-muted text-muted",
  synced: "border-success text-success",
  failed: "border-danger text-danger",
};

export interface SightingCardProps {
  sighting: LocalSighting;
  birdName?: string;
  onOpen?: (id: string) => void;
  onDelete?: (id: string) => Promise<void>;
}

export function SightingCard({
  sighting,
  birdName,
  onOpen,
  onDelete,
}: SightingCardProps) {
  const [confirming, setConfirming] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const handleConfirmDelete = async () => {
    if (!onDelete) return;
    setDeleting(true);
    try {
      await onDelete(sighting.id);
      // No reset on success: the row leaves liveSightings() and this card unmounts.
    } catch {
      setDeleting(false);
    }
  };

  return (
    <li className="flex flex-col gap-3 rounded-lg border border-slate-200 p-4">
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          {sighting.photoPaths.length > 0 && (
            <PhotoThumb path={sighting.photoPaths[0]} birdName={birdName} />
          )}
          <div>
            <p className="text-lg font-semibold text-ink">
              {birdName ?? sighting.quickNote ?? "Unidentified bird"}
            </p>
            <p className="text-muted">
              {formatObservedAt(
                sighting.observedAt,
                sighting.observedAtOffsetMinutes,
              )}
            </p>
            <p className="text-muted">
              {sighting.latitude === undefined
                ? "No location recorded"
                : "Location recorded"}
            </p>
          </div>
        </div>
        <Badge variant="outline" className={SYNC_CLASS[sighting.syncStatus]}>
          {SYNC_LABEL[sighting.syncStatus]}
        </Badge>
      </div>

      {sighting.recordingPaths.length > 0 && (
        <RecordingPlayer path={sighting.recordingPaths[0]} label="Recording" />
      )}

      {confirming ? (
        <div className="space-y-3">
          <p className="text-lg text-ink">
            Delete this sighting? It will be removed from all your devices.
          </p>
          <div className="flex gap-3">
            <button
              type="button"
              onClick={() => setConfirming(false)}
              disabled={deleting}
              className="h-12 flex-1 rounded-md border border-slate-300 bg-white text-base font-medium text-ink disabled:opacity-50"
            >
              Keep it
            </button>
            <button
              type="button"
              onClick={() => void handleConfirmDelete()}
              disabled={deleting}
              className="h-12 flex-1 rounded-md bg-danger text-base font-medium text-white disabled:opacity-50"
            >
              {deleting ? "Deleting…" : "Delete"}
            </button>
          </div>
        </div>
      ) : (
        (onOpen || onDelete) && (
          <div className="flex gap-3">
            {onOpen && (
              <button
                type="button"
                onClick={() => onOpen(sighting.id)}
                className="h-12 flex-1 rounded-md border border-primary text-base font-medium text-primary"
              >
                Add Species or Notes
              </button>
            )}
            {onDelete && (
              <button
                type="button"
                onClick={() => setConfirming(true)}
                className="h-12 w-24 rounded-md border border-danger text-base font-medium text-danger"
              >
                Delete
              </button>
            )}
          </div>
        )
      )}
    </li>
  );
}
