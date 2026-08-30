import { db } from "@/db/db";
import { useLiveQuery } from "dexie-react-hooks";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { QuickTag } from "./QuickTag";
import { MAX_RECORDINGS, RecordingCapture } from "./RecordingCapture";
import { RecordingPlayer } from "./RecordingPlayer";

export interface CaptureSheetProps {
  sightingId: string;
  open: boolean;
  onClose: () => void;
}

export function CaptureSheet({ sightingId, open, onClose }: CaptureSheetProps) {
  const row = useLiveQuery(() => db.sightings.get(sightingId), [sightingId]);
  const queuedRecordings = useLiveQuery(
    () =>
      db.recordings
        .where("sightingId")
        .equals(sightingId)
        .and((r) => r.uploaded === 0)
        .toArray(),
    [sightingId],
  );

  if (!row || row.deleted || queuedRecordings === undefined) return null;

  const remaining =
    MAX_RECORDINGS - (row.recordingPaths.length + queuedRecordings.length);

  return (
    <Dialog
      open={open}
      onOpenChange={(isOpen) => {
        if (!isOpen) onClose();
      }}
    >
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Sighting saved</DialogTitle>
          <DialogDescription>
            {row.latitude === undefined
              ? "Saved without location."
              : "Saved with your location."}{" "}
            Add details now, or close this and come back to it later from
            Sightings.
          </DialogDescription>
        </DialogHeader>
        <QuickTag sightingId={row.id} />
        <div>
          <p className="mb-1 font-medium text-ink">Sound recording</p>
          {(row.recordingPaths.length > 0 || queuedRecordings.length > 0) && (
            <ul className="mb-3 flex flex-col gap-2">
              {row.recordingPaths.map((path) => (
                <li key={path}>
                  <RecordingPlayer path={path} label="Recording" />
                </li>
              ))}
              {queuedRecordings.map((recording) => (
                <li key={recording.id}>
                  <RecordingPlayer blob={recording.blob} label="Recording" />
                </li>
              ))}
            </ul>
          )}
          <RecordingCapture sightingId={row.id} remaining={remaining} />
        </div>
        <DialogFooter>
          <Button type="button" className="h-12 w-full" onClick={onClose}>
            Done
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
