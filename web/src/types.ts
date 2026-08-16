export type SyncStatus = "pending" | "synced" | "failed";

export interface LocalSighting {
  id: string;
  observedAt: string;
  observedAtOffsetMinutes: number;
  clientUpdatedAt: string;
  birdId?: string;
  quickNote?: string;
  notes?: string;
  latitude?: number;
  longitude?: number;
  accuracyM?: number; // GPS fix's uncertainty radius
  photoPaths: string[]; // Storage paths the server knows about
  deleted?: 0 | 1; // tombstone; 0|1 like LocalPhoto.uploaded in case it's ever indexed
  syncStatus: SyncStatus;
  syncError?: string;
}
