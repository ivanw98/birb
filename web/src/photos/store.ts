import { db } from "@/db/db";

export interface PhotoStore {
  upload(sightingId: string, fileName: string, blob: Blob): Promise<string>;
  getRemoteUrl(path: string): Promise<string | null>;
}

// dev store
export const devPhotoStore: PhotoStore = {
  async upload(sightingId, fileName) {
    const uid = import.meta.env.VITE_DEV_AUTH_UID ?? "dev";
    return `${uid}/${sightingId}/${fileName}`;
  },

  async getRemoteUrl() {
    return null;
  },
};

// Look up a locally-held blob for a path (any store)
export async function localBlobForPath(
  path: string,
): Promise<Blob | undefined> {
  const [_, sightingId, filename] = path.split("/");
  if (!sightingId || !filename) return undefined;

  const row = await db.photos
    .where("sightingId")
    .equals(sightingId)
    .and((p) => p.fileName === filename)
    .first();

  return row?.blob;
}
