import { db } from "@/db/db";
import type { components } from "./schema";
import { apiClient } from "./client";

export type Bird = components["schemas"]["Bird"];
export type Sighting = components["schemas"]["Sighting"];

const ETAG_KEY = "birdsEtag";

// Refresh the cached UK bird list into Dexie.
export async function refreshBirds(): Promise<void> {
  const haveAny = (await db.birds.count()) > 0;
  const meta = haveAny ? await db.meta.get(ETAG_KEY) : undefined;

  const { data, response } = await apiClient.GET("/api/birds", {
    headers: meta ? { "If-None-Match": meta.value } : {},
  });

  if (response.status === 304) return;

  if (!data) throw new Error(`bird list fetch failed: ${response.status}`);

  const etag = response.headers.get("ETag");
  const scope = async () => {
    await db.birds.clear();
    await db.birds.bulkPut(data);

    if (etag) await db.meta.put({ key: ETAG_KEY, value: etag });
  };
  await db.transaction("rw", db.birds, db.meta, scope);
}
