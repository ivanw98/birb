import { useCallback, useMemo, useState } from "react";
import { useLiveQuery } from "dexie-react-hooks";
import { db } from "@/db/db";
import { EnrichSighting } from "./EnrichSighting";
import { SightingList } from "./SightingList";
import { SightingMap } from "./SightingMap";

type SightingsMode = "list" | "map";

const MODES: Array<{ mode: SightingsMode; label: string }> = [
  { mode: "list", label: "List" },
  { mode: "map", label: "Map" },
];

export function SightingsView() {
  const [mode, setMode] = useState<SightingsMode>("list");
  const [enrichingId, setEnrichingId] = useState<string | null>(null);

  const sightings = useLiveQuery(() => db.sightings.toArray()) ?? [];
  // `?? []` goes inside the memo, not on the `birds` line. While the query is
  // still undefined, a fallback literal there is a new reference every render
  // and the memo re-runs; as a dep, `Bird[] | undefined` is stable throughout.
  const birds = useLiveQuery(() => db.birds.toArray());
  const birdNames = useMemo(
    () => new Map((birds ?? []).map((bird) => [bird.id, bird.commonName])),
    [birds],
  );
  const birdNameFor = useCallback(
    (birdId: string | undefined) =>
      (birdId && birdNames.get(birdId)) || "Unidentified bird",
    [birdNames],
  );

  return (
    <div className="flex flex-col gap-4 pt-4">
      <div
        role="group"
        aria-label="Sightings view"
        className="inline-flex self-center overflow-hidden rounded-lg border border-slate-300"
      >
        {MODES.map((item) => {
          const isActive = mode === item.mode;
          return (
            <button
              key={item.mode}
              type="button"
              aria-pressed={isActive}
              onClick={() => setMode(item.mode)}
              className={`h-12 min-w-32 border-l border-slate-300 px-6 text-lg font-medium first:border-l-0 ${
                isActive ? "bg-primary text-white" : "bg-white text-ink"
              }`}
            >
              {item.label}
            </button>
          );
        })}
      </div>

      {mode === "list" ? (
        <SightingList onOpen={setEnrichingId} />
      ) : (
        <SightingMap sightings={sightings} birdNameFor={birdNameFor} />
      )}

      {enrichingId && (
        <EnrichSighting
          sightingId={enrichingId}
          onClose={() => setEnrichingId(null)}
        />
      )}
    </div>
  );
}
