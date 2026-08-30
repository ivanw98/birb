import { db } from "@/db/db";

export interface RecordingStore {
  upload(
    sightingId: string,
    fileName: string,
    blob: Blob,
    mimeType: string,
  ): Promise<string>;
  getRemoteUrl(path: string): Promise<string | null>;
}

export async function localRecordingBlobForPath(
  path: string,
): Promise<Blob | undefined> {
  const [_, sightingId, fileName] = path.split("/");
  if (!sightingId || !fileName) return undefined;

  const row = await db.recordings
    .where("sightingId")
    .equals(sightingId)
    .and((r) => r.fileName === fileName)
    .first();

  return row?.blob;
}
