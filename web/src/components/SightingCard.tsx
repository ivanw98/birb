import { Badge } from "@/components/ui/badge";
import { formatObservedAt } from "../lib/time";
import type { LocalSighting, SyncStatus } from "../types";
import { PhotoThumb } from "./PhotoThumb";

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
}

export function SightingCard({
  sighting,
  birdName,
  onOpen,
}: SightingCardProps) {
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

      {onOpen && (
        <button
          type="button"
          onClick={() => onOpen(sighting.id)}
          className="h-12 w-full rounded-md border border-primary text-base font-medium text-primary"
        >
          Add Species or Notes
        </button>
      )}
    </li>
  );
}
