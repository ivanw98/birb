import { useCallback, useMemo, useState } from "react";
import { useLiveQuery } from "dexie-react-hooks";
import { db, liveSightings } from "@/db/db";
import { deleteSighting, undoDelete } from "@/lib/enrich";
import { EnrichSighting } from "./EnrichSighting";
import { SightingList } from "./SightingList";
import { SightingMap } from "./SightingMap";
import { StatusBanner } from "./StatusBanner";

type SightingsMode = "list" | "map";

const MODES: Array<{ mode: SightingsMode; label: string }> = [
  { mode: "list", label: "List" },
  { mode: "map", label: "Map" },
];

// The most recent deletion, backing the persistent Undo banner. remote=true
// means it happened on another device and was discovered via a 404.
interface DeletedNotice {
  id: string;
  remote: boolean;
}

export function SightingsView() {
  const [mode, setMode] = useState<SightingsMode>("list");
  const [enrichingId, setEnrichingId] = useState<string | null>(null);
  const [deleted, setDeleted] = useState<DeletedNotice | null>(null);

  const sightings = useLiveQuery(() => liveSightings()) ?? [];
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
      {deleted && (
        <div className="self-center">
          <StatusBanner
            tone={deleted.remote ? "info" : "success"}
            onDismiss={() => setDeleted(null)}
          >
            <span>
              {deleted.remote
                ? "That sighting was deleted on another device."
                : "Sighting deleted."}
            </span>
            <button
              type="button"
              className="ml-3 h-12 rounded-md bg-primary px-4 text-white"
              onClick={() => {
                void undoDelete(deleted.id);
                setDeleted(null);
              }}
            >
              Undo
            </button>
          </StatusBanner>
        </div>
      )}

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
        <SightingList
          onOpen={setEnrichingId}
          onDelete={async (id) => {
            await deleteSighting(id);
            setDeleted({ id, remote: false });
          }}
        />
      ) : (
        <SightingMap sightings={sightings} birdNameFor={birdNameFor} />
      )}

      {enrichingId && (
        <EnrichSighting
          sightingId={enrichingId}
          onClose={() => setEnrichingId(null)}
          onDeleted={(id, remote) => {
            setEnrichingId(null);
            setDeleted({ id, remote });
          }}
        />
      )}
    </div>
  );
}
