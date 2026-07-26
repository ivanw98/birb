import { db } from "@/db/db";
import { useLiveQuery } from "dexie-react-hooks";
import { useMemo, useState } from "react";

const MAX_RESULTS = 8;

// a sighting doesn't have to have a species identified and birdId can be unset
export interface BirdPickerProps {
  birdId?: string;
  onChange: (birdId: string | undefined) => void;
}

export function BirdPicker({ birdId, onChange }: BirdPickerProps) {
  const [query, setQuery] = useState("");
  const birdQuerier = () => db.birds.toArray(); // get whole birds array in memory
  const birds = useLiveQuery(birdQuerier, []);
  const chosen = birdId ? birds?.find((bird) => bird.id === birdId) : undefined;

  const memoFactory = () => {
    const q = query.trim().toLocaleLowerCase();
    if (!birds || q === "") return [];

    return birds
      .filter(
        (bird) =>
          bird.commonName.toLocaleLowerCase().includes(q) ||
          bird.scientificName.toLocaleLowerCase().includes(q),
      )
      .slice(0, MAX_RESULTS);
  };

  // useMemo is largely replaced by ReactCompiler (not enabled currently)
  // useMemo is still needed where identity is semantics
  // (context values, dependency inputs) rather than performance.
  const results = useMemo(memoFactory, [birds, query]);

  if (birdId) {
    // Confirmation state: once a species is chosen, stop showing the search  box entirely
    return (
      <div className="rounded-lg border border-slate-300 p-3">
        <p className="text-lg text-ink">
          {chosen ? (
            <>
              <span className="font-semibold">{chosen.commonName}</span>{" "}
              <span className="italic text-muted">{chosen.scientificName}</span>
            </>
          ) : (
            "Loading species..."
          )}
        </p>
        <button
          type="button"
          onClick={() => onChange(undefined)}
          className="mt-2 h-12 rounded-md border border-slate-400 px-4 text-base font-medium text-ink"
        >
          Clear Species
        </button>
      </div>
    );
  }

  return (
    <div>
      <label
        htmlFor="bird-picker-query"
        className="mb-2 block font-medium text-ink"
      >
        Search species
      </label>
      <input
        id="bird-picker-query"
        type="text"
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        placeholder="e.g. wren, or Troglodytes"
        className="h-12 w-full rounded-md border border-slate-400 px-3 text-lg"
        autoComplete="off"
      />
      {/* aria-live: sighted users see the buttons appear; screen reader users get a spoken count */}
      <p aria-live="polite" className="mt-2 min-h-6 text-sm text-muted">
        {query.trim() === ""
          ? "Type at least part of a name to search."
          : results.length === 0
            ? `No species match "${query.trim()}".`
            : `${results.length} species found.`}
      </p>
      <div className="mt-2 grid gap-2">
        {results.map((bird) => (
          <button
            key={bird.id}
            type="button"
            onClick={() => onChange(bird.id)}
            className="h-14 rounded-md border border-slate-400 px-4 text-left text-lg text-ink"
          >
            {bird.commonName} —{" "}
            <span className="italic text-muted">{bird.scientificName}</span>
          </button>
        ))}
      </div>
    </div>
  );
}
