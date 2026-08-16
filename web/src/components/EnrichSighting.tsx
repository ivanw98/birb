import { db } from "@/db/db";
import { useLiveQuery } from "dexie-react-hooks";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./ui/dialog";
import type { LocalSighting } from "@/types";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { BirdPicker } from "./BirdPicker";
import { Label } from "./ui/label";
import { Textarea } from "./ui/textarea";
import { Button } from "./ui/button";
import { Input } from "./ui/input";
import {
  removePhoto,
  saveEnrichment,
  type PhotoTarget,
  type SightingContent,
} from "@/lib/enrich";
import { StatusBanner } from "./StatusBanner";
import { syncSettled } from "@/sync/syncEngine";
import { MAX_PHOTOS, PhotoCapture } from "./PhotoCapture";
import { PhotoThumb } from "./PhotoThumb";

const QUICK_NOTE_MAX = 280;
const NOTES_MAX = 5000;

export interface EnrichSightingProps {
  sightingId: string;
  onClose: () => void;
  // Fired when a save/remove hits a 404: the sighting was deleted on another
  // device. The parent closes the dialog and owns the Undo banner.
  onDeleted: (id: string, remote: boolean) => void;
}

//EnrichSighting is a component whose only job is turning a sightingId into a loaded row
export function EnrichSighting({
  sightingId,
  onClose,
  onDeleted,
}: EnrichSightingProps) {
  const row = useLiveQuery(() => db.sightings.get(sightingId), [sightingId]);

  // Distinguishes still-loading `undefined` from a row that existed then vanished.
  const hadRow = useRef(false);
  useEffect(() => {
    if (row) hadRow.current = true;
    else if (hadRow.current) onDeleted(sightingId, true);
  }, [row, sightingId, onDeleted]);

  return (
    <Dialog
      /* open is a bare boolean prop that is always true. */
      open
      /*  it fires with false whenever Escape is pressed, the backdrop is clicked, or (later) a close button is clicked */
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Enrich sighting</DialogTitle>
          <DialogDescription>
            Add a species and notes, or edit what's there.
          </DialogDescription>
        </DialogHeader>
        {row ? (
          <EnrichForm row={row} onClose={onClose} onDeleted={onDeleted} />
        ) : (
          <p className="text-muted">Loading...</p>
        )}
      </DialogContent>
    </Dialog>
  );
}

// A thumbnail in the strip is either a path the server knows about or a blob
// still sitting in the local queue. `key` doubles as React's key and the
// in-flight marker for the Remove button.
type PhotoItem =
  | { key: string; kind: "attached"; path: string }
  | { key: string; kind: "queued"; id: number; blob: Blob };

interface EnrichFormProps {
  row: LocalSighting;
  onClose: () => void;
  onDeleted: (id: string, remote: boolean) => void;
}

function EnrichForm({ row, onClose, onDeleted }: EnrichFormProps) {
  const [birdId, setBirdId] = useState(row.birdId);
  const [quickNote, setQuickNote] = useState(row.quickNote ?? "");
  const [notes, setNotes] = useState(row.notes ?? "");
  const [saving, setSaving] = useState(false);
  const [banner, setBanner] = useState<string | null>(null);
  // The key of the photo currently being removed, or null. One at a time: an
  // attached removal is a PUT, and two in flight would race on clientUpdatedAt.
  const [removingKey, setRemovingKey] = useState<string | null>(null);

  const queuedPhotos =
    useLiveQuery(
      () =>
        db.photos
          .where("sightingId")
          .equals(row.id)
          .and((p) => p.uploaded === 0)
          .toArray(),
      [row.id],
    ) ?? [];

  const remaining = MAX_PHOTOS - (row.photoPaths.length + queuedPhotos.length);

  // One strip for both kinds, so attaching a photo is visibly confirmed even
  // offline — before this, thumbnails came only from row.photoPaths, so a photo
  // queued with no connection showed nothing at all until sync caught up.
  const photoItems: PhotoItem[] = [
    ...row.photoPaths.map((path) => ({
      key: path,
      kind: "attached" as const,
      path,
    })),
    ...queuedPhotos.map((photo) => ({
      key: `queued-${photo.id}`,
      kind: "queued" as const,
      id: photo.id!,
      blob: photo.blob,
    })),
  ];

  const handleRemove = async (item: PhotoItem) => {
    setRemovingKey(item.key);
    setBanner(null);
    const target: PhotoTarget =
      item.kind === "attached"
        ? { kind: "attached", path: item.path }
        : { kind: "queued", id: item.id };

    const result = await removePhoto(row, target);
    setRemovingKey(null);

    if (result.outcome === "removed") return;
    if (result.outcome === "offline") {
      setBanner(
        "Removing a photo that's already uploaded needs a connection. Try again once you're back online.",
      );
      return;
    }
    if (result.outcome === "conflict") {
      setBanner(
        "Updated elsewhere — showing the latest. Check the photos and try again if it's still there.",
      );
      return;
    }
    if (result.outcome === "gone") {
      onDeleted(row.id, true);
      return;
    }
    setBanner(result.message);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setBanner(null);

    const content: SightingContent = {
      birdId,
      quickNote: normalise(quickNote),
      notes: normalise(notes),
    };

    // Attaching a photo fires syncNow(), and syncPhotos() PUTs this same
    // sighting with a bumped clientUpdatedAt. Saving on top of a pass that's
    // still in flight means one of the two 409s — and ours doesn't retry, by
    // design, so the user's text would be the loser of a conflict they caused
    // themselves. Wait the pass out, then re-read: `row` was captured at render
    // and its clientUpdatedAt/photoPaths may both be a version behind.
    await syncSettled();
    const fresh = (await db.sightings.get(row.id)) ?? row;

    const result = await saveEnrichment(fresh, content);
    setSaving(false);

    if (
      result.outcome === "saved-offline" ||
      result.outcome === "saved-online"
    ) {
      onClose();
      return;
    }

    if (result.outcome === "conflict") {
      setBanner(
        "Updated elsewhere — showing the latest; your text is still in the form.",
      );
      return;
    }
    if (result.outcome === "gone") {
      onDeleted(row.id, true);
      return;
    }
    setBanner(result.message);
  };

  return (
    <form onSubmit={(e) => void handleSubmit(e)} className="space-y-4">
      {banner && <StatusBanner tone="danger">{banner}</StatusBanner>}
      <div>
        <p className="mb-1 font-medium text-ink">Species</p>
        <BirdPicker birdId={birdId} onChange={setBirdId} />
      </div>

      <div>
        <Label htmlFor="enrich-quick-note">Quick note</Label>
        <Input
          id="enrich-quick-note"
          value={quickNote}
          maxLength={QUICK_NOTE_MAX}
          onChange={(event) => setQuickNote(event.target.value)}
          placeholder="e.g. small brown bird in reeds"
        />
        <p className="mt-2 text-sm text-muted">
          {quickNote.length}/{QUICK_NOTE_MAX} characters
        </p>
      </div>

      <div>
        <Label htmlFor="enrich-notes">Notes</Label>
        <Textarea
          id="enrich-notes"
          value={notes}
          maxLength={NOTES_MAX}
          onChange={(event) => setNotes(event.target.value)}
          rows={5}
          placeholder="Longer notes from back at base"
          className="max-h-48"
        />
        <p className="mt-2 text-sm text-muted">
          {notes.length}/{NOTES_MAX} characters
        </p>
      </div>
      <div>
        <p className="mb-1 font-medium text-ink">Photos </p>
        <p className="mb-2 text-sm text-muted">
          Photos save as soon as you add them. Cancel won't undo that — use
          Remove.
        </p>
        {photoItems.length > 0 && (
          <ul className="mb-3 flex flex-wrap gap-3">
            {photoItems.map((item, index) => (
              <li key={item.key} className="flex flex-col items-center gap-1">
                {item.kind === "attached" ? (
                  <PhotoThumb path={item.path} />
                ) : (
                  <PhotoThumb blob={item.blob} />
                )}
                <button
                  type="button"
                  onClick={() => void handleRemove(item)}
                  disabled={removingKey !== null}
                  aria-label={`Remove photo ${index + 1} of ${photoItems.length}`}
                  className="h-12 w-24 rounded-md border border-danger text-base font-medium text-danger disabled:opacity-50"
                >
                  {removingKey === item.key ? "Removing…" : "Remove"}
                </button>
              </li>
            ))}
          </ul>
        )}
        <PhotoCapture
          sightingId={row.id}
          remaining={remaining}
          disabled={saving || removingKey !== null}
        ></PhotoCapture>
      </div>

      <DialogFooter>
        <Button
          type="button"
          variant="outline"
          onClick={onClose}
          className="h-12 px-6 text-base"
        >
          Cancel
        </Button>
        <Button type="submit" disabled={saving} className="h-12 px-6 text-base">
          {saving ? "Saving…" : "Save"}
        </Button>
      </DialogFooter>
    </form>
  );
}

function normalise(value: string): string | undefined {
  return value.trim() === "" ? undefined : value.trim();
}
