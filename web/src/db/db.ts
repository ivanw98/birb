import type { Bird } from "@/api/birds";
import type { LocalSighting } from "@/types";
import Dexie, { type Table, type Transaction } from "dexie";

export interface MetaEntry {
  key: string;
  value: string;
}

// Added in migration(2)
export interface LocalPhoto {
  id?: number; // auto-increment
  sightingId: string;
  fileName: string; // "<ulid>.jpg" (a path segment, not a full path)
  blob: Blob; // compressed JPEG, ~200KB; NEVER the camera original
  uploaded: 0 | 1; // number, not boolean: IndexedDB can't index booleans
}

// added in migration(3); id is the wire sightingId
export interface FeedRow {
  id: string;
  birdId?: string;
  authorName?: string;
  observedAt: string;
  placeName?: string;
  photoPaths: string[];
  recordingPaths: string[];
}

// added in migration(4);
export interface LocalRecording {
  id?: number;
  sightingId: string;
  fileName: string;
  blob: Blob;
  mimeType: string;
  uploaded: 0 | 1;
}

export class BirbDB extends Dexie {
  sightings!: Table<LocalSighting, string>;
  birds!: Table<Bird, string>;
  meta!: Table<MetaEntry, string>;
  // added in migration(2)
  photos!: Table<LocalPhoto, number>;
  // added in migration(3)
  feedItems!: Table<FeedRow, string>;
  // added in migration(4)
  recordings!: Table<LocalRecording, number>;
  constructor() {
    // Call Dexie constructor and pass `birb` as the db name
    super("birb");

    // tables: PK + indexes
    const schema_1 = {
      sightings: "id, syncStatus, observedAt",
      birds: "id, commonName",
      meta: "key",
    };

    this.version(1).stores(schema_1);

    const schema_2 = {
      photos: "++id, sightingId, uploaded",
    };

    this.version(2).stores(schema_2);

    const schema_3 = {
      feedItems: "id, observedAt",
    };

    this.version(3).stores(schema_3);

    const schema_4 = {
      recordings: "++id, sightingId, uploaded",
    };

    // recordingPaths is new on the sightings row, and so rows already exist on disk that predate it
    const backfillMediaPaths = async (tx: Transaction) => {
      await tx
        .table("sightings")
        .toCollection()
        .modify((sighting: Partial<LocalSighting>) => {
          if (sighting.recordingPaths === undefined)
            sighting.recordingPaths = [];
        });

      await tx
        .table("feedItems")
        .toCollection()
        .modify((feed: Partial<FeedRow>) => {
          if (feed.photoPaths === undefined) feed.photoPaths = [];
          if (feed.recordingPaths === undefined) feed.recordingPaths = [];
        });
    };

    this.version(4).stores(schema_4).upgrade(backfillMediaPaths);
  }
}

export const db = new BirbDB();

// Tombstoned rows stay in Dexie for sync (and Undo) but never render; every
// display read goes through here.
export function liveSightings(): Promise<LocalSighting[]> {
  return db.sightings
    .orderBy("observedAt")
    .reverse()
    .filter((s) => !s.deleted)
    .toArray();
}

// It is not possible to write db.sightings.where("birdId").equals(birdId) as Dexie does not index birdId for sightings
// this.version(2).stores({ sightings: "id, syncStatus, observedAt, birdId" }); needed
export function sightingsForBird(birdID: string): Promise<LocalSighting[]> {
  const filter = (s: LocalSighting) => !s.deleted && s.birdId === birdID;
  return db.sightings.orderBy("observedAt").reverse().filter(filter).toArray();
}

export function liveFeed(): Promise<FeedRow[]> {
  return db.feedItems.orderBy("observedAt").reverse().toArray();
}
