import { db, type FeedRow } from "@/db/db";
import { apiClient } from "./client";
import type { components } from "./schema";
import { nowISO } from "@/lib/time";

export const FEED_SYNCED_KEY = "feedLastSyncedAt";
export type FeedItem = components["schemas"]["FeedItem"];

function toRow(item: FeedItem): FeedRow {
  return {
    id: item.sightingId,
    birdId: item.birdId,
    authorName: item.authorName,
    observedAt: item.observedAt,
    placeName: item.placeName,
    photoPaths: item.photoPaths,
    recordingPaths: item.recordingPaths,
  };
}

export async function refreshFeed(): Promise<void> {
  const items: FeedItem[] = [];
  let cursor: string | undefined;
  do {
    const { data, response } = await apiClient.GET("/api/feed", {
      params: { query: { limit: 100, cursor } },
    });

    if (!data) throw new Error(`feed fetch failed: ${response.status}`);
    items.push(...data.items);
    cursor = data.nextCursor ?? undefined;
  } while (cursor && items.length < 500);

  const scope = async () => {
    await db.feedItems.clear();
    await db.feedItems.bulkPut(items.map(toRow));
    await db.meta.put({ key: FEED_SYNCED_KEY, value: nowISO() });
  };

  await db.transaction("rw", db.feedItems, db.meta, scope);
}
