import { useLiveQuery } from "dexie-react-hooks";
import { db, liveFeed } from "@/db/db";
import { FEED_SYNCED_KEY, refreshFeed } from "@/api/feed";
import { syncSettled } from "@/sync/syncEngine";

export function useFeed() {
  const items = useLiveQuery(() => liveFeed());
  const lastSynced = useLiveQuery(() => db.meta.get(FEED_SYNCED_KEY));

  const refresh = async (): Promise<void> => {
    await syncSettled();
    await refreshFeed();
  };

  return { items, lastSyncedAt: lastSynced?.value, refresh };
}
