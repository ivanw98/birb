import { SightingCard } from "./SightingCard";
import { useLiveQuery } from "dexie-react-hooks";
import { db, liveSightings } from "@/db/db";
import { storageEstimate } from "../lib/storage";
import type { Bird } from "@/api/birds";
import { useEffect, useState } from "react";

const birdFn = (bird: Bird): [string, string] => [bird.id, bird.commonName];

export interface SightingListProps {
  onOpen: (id: string) => void;
  onDelete: (id: string) => Promise<void>;
}

export function SightingList({ onOpen, onDelete }: SightingListProps) {
  const sightings = useLiveQuery(() => liveSightings());

  const birds = useLiveQuery(() => db.birds.toArray());
  const birdNames = new Map<string, string>((birds ?? []).map(birdFn));

  const [estimate, setEstimate] = useState<{
    usageMB: number;
    quotaMB: number;
  } | null>(null);

  // no deps because storageEstimate is defined outside of any component
  // it doesn't read any props, or component state and its memory ref is immutable
  useEffect(() => {
    void storageEstimate().then(setEstimate);
  }, []);

  if (sightings === undefined) {
    return <p className="p-4 text-muted">Loading your sightings...</p>;
  }
  if (sightings.length === 0) {
    return (
      <p className="p-4 text-muted">
        No sightings yet. Press Record to add your first one.
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <ul className="flex flex-col gap-3 p-4">
        {sightings.map((sighting) => (
          <SightingCard
            key={sighting.id}
            sighting={sighting}
            birdName={
              sighting.birdId ? birdNames.get(sighting.birdId) : undefined
            }
            onOpen={onOpen}
            onDelete={onDelete}
          />
        ))}
      </ul>
      {estimate && (
        <p className="text-xs text-muted">
          Using about {estimate.usageMB.toFixed(1)} MB of an estimated{" "}
          {Math.round(estimate.quotaMB)} MB available on this device.
        </p>
      )}
    </div>
  );
}
