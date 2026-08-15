import { db } from "@/db/db";
import { quickTag } from "@/lib/quickTag";
import type { LocalSighting } from "@/types";
import { useLiveQuery } from "dexie-react-hooks";
import { useState } from "react";
import { StatusBanner } from "./StatusBanner";
import { BirdPicker } from "./BirdPicker";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

const QUICK_NOTE_MAX = 280;

export interface QuickTagProps {
  sightingId: string;
}

export function QuickTag({ sightingId }: QuickTagProps) {
  const sightingQuerier = () => db.sightings.get(sightingId);
  const row = useLiveQuery(sightingQuerier, [sightingId]);
  // A tombstoned row is gone as far as any editor is concerned — a quick-tag
  // write against it would be lost when the delete syncs.
  return row && !row.deleted ? <QuickTagForm key={row.id} row={row} /> : null;
}

interface QuickTagFormProps {
  row: LocalSighting;
}

function QuickTagForm({ row }: QuickTagFormProps) {
  // seeded once: row keeps arriving fresh from useLiveQuery
  const [birdId, setBirdId] = useState(row.birdId);
  const [quickNote, setQuickNote] = useState(row.quickNote ?? "");
  const [tagError, setTagError] = useState<string | null>(null);

  const handleBirdChange = async (id: string | undefined) => {
    setTagError(null);
    try {
      await quickTag(row, { birdId: id });

      setBirdId(id);
    } catch (err) {
      setTagError(err instanceof Error ? err.message : String(err));
    }
  };

  const handleNoteBlur = async () => {
    const value = quickNote.trim() === "" ? undefined : quickNote.trim();

    // blurring without editing is the common case
    if (value === row.quickNote) return;
    setTagError(null);
    try {
      await quickTag(row, { quickNote: value });
    } catch (err) {
      setTagError(err instanceof Error ? err.message : String(err));
    }
  };

  return (
    <div className="flex w-full max-w-md flex-col gap-3">
      {tagError && <StatusBanner tone="danger">{tagError}</StatusBanner>}
      <div>
        <p className="mb-1 font-medium text-ink">Species</p>
        <BirdPicker
          birdId={birdId}
          onChange={(id) => void handleBirdChange(id)}
        />
      </div>
      <div>
        <Label htmlFor="quick-tag-note">Quick note</Label>
        <Input
          id="quick-tag-note"
          value={quickNote}
          maxLength={QUICK_NOTE_MAX}
          onChange={(event) => setQuickNote(event.target.value)}
          onBlur={() => void handleNoteBlur()}
          placeholder="e.g. small brown bird in reeds"
        />
      </div>
    </div>
  );
}
