import { useEffect, useState } from "react";
import { useLiveQuery } from "dexie-react-hooks";
import { db, type FeedRow } from "@/db/db";
import { refreshBirds } from "@/api/birds";
import { formatFeedTime } from "@/lib/time";
import { useFeed } from "@/hooks/useFeed";
import { useGroups } from "@/hooks/useGroups";
import { useOnline } from "@/hooks/useOnline";
import { StatusBanner } from "./StatusBanner";
import { GroupsView } from "./GroupsView";
import { PhotoThumb } from "./PhotoThumb";
import { RecordingPlayer } from "./RecordingPlayer";

const STALE_AFTER_MS = 60 * 60 * 1000;

function rowText(row: FeedRow, birdNames: Map<string, string>): string {
  const species =
    (row.birdId && birdNames.get(row.birdId)) || "Unidentified bird";
  const author = row.authorName ?? "A group member";
  const place = row.placeName ? `, near ${row.placeName}` : "";
  return `${species} - ${author}${place}, ${formatFeedTime(row.observedAt)}`;
}

export function FeedView() {
  const { items, lastSyncedAt, refresh } = useFeed();
  const { groups } = useGroups();
  const online = useOnline();
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showGroups, setShowGroups] = useState<boolean>(false);

  const birds = useLiveQuery(() => db.birds.toArray());
  const birdNames = new Map((birds ?? []).map((b) => [b.id, b.commonName]));

  // Wait for the birds query to resolve, or an empty map looks like a stale cache.
  const staleBirdCache =
    birds !== undefined &&
    (items ?? []).some((row) => row.birdId && !birdNames.has(row.birdId));

  useEffect(() => {
    if (staleBirdCache) void refreshBirds().catch(() => {});
  }, [staleBirdCache]);

  const handleRefresh = async () => {
    setRefreshing(true);
    setError(null);
    try {
      await refresh();
    } catch {
      setError(
        "Could not update the feed. Check your connection and try again.",
      );
    } finally {
      setRefreshing(false);
    }
  };

  if (showGroups) {
    return <GroupsView onBack={() => setShowGroups(false)} />;
  }

  if (items === undefined) {
    return <p className="p-4 text-muted">Loading the feed...</p>;
  }

  const staleFeed =
    lastSyncedAt === undefined ||
    Date.now() - Date.parse(lastSyncedAt) > STALE_AFTER_MS;
  const inGroups = groups.status === "success" && groups.data.length > 0;

  return (
    <div className="flex w-full max-w-md flex-col gap-4 self-center">
      {(!online || staleFeed) && (
        <StatusBanner tone="info">
          {lastSyncedAt
            ? `Last updated ${formatFeedTime(lastSyncedAt)}.`
            : "Not updated yet."}
        </StatusBanner>
      )}

      {error && (
        <StatusBanner tone="danger" onDismiss={() => setError(null)}>
          {error}
        </StatusBanner>
      )}

      <button
        type="button"
        onClick={() => void handleRefresh()}
        disabled={refreshing}
        className="h-12 rounded-md bg-primary px-4 text-lg font-medium text-white disabled:opacity-50"
      >
        {refreshing ? "Refreshing…" : "Refresh"}
      </button>

      {items.length === 0 ? (
        <p className="p-4 text-muted">
          {inGroups || groups.status !== "success"
            ? "No sightings from your friends in the last 7 days."
            : "You're not in any groups yet. Open Friends & groups below to create or join one."}
        </p>
      ) : (
        <ul className="flex flex-col gap-3">
          {items.map((row) => (
            <li
              key={row.id}
              className="rounded-lg border border-slate-200 p-4 text-lg text-ink"
            >
              <p>{rowText(row, birdNames)}</p>
              {(row.photoPaths.length > 0 || row.recordingPaths.length > 0) && (
                <div className="flex flex-wrap items-center gap-3">
                  {row.photoPaths.length > 0 && (
                    <PhotoThumb path={row.photoPaths[0]} />
                  )}
                  {row.recordingPaths.length > 0 && (
                    <RecordingPlayer
                      path={row.recordingPaths[0]}
                      label="Recording"
                    />
                  )}
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
      <button
        type="button"
        onClick={() => setShowGroups(true)}
        className="h-12 rounded-md border border-slate-300 bg-white text-lg font-medium text-ink"
      >
        Friends &amp; groups
      </button>
    </div>
  );
}
