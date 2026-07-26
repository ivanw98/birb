import { db } from "@/db/db";
import { useOnline } from "@/hooks/useOnline";
import { useSyncState } from "@/hooks/useSyncState";
import { syncNow, type SyncResult } from "@/sync/syncEngine";
import { useLiveQuery } from "dexie-react-hooks";
import { Button } from "./ui/button";

function describeResult(result: SyncResult | null): string {
  if (!result) return "Not synced yet this session.";

  switch (result.ok) {
    case true:
      return result.failed > 0
        ? `Synced ${result.pushed}, ${result.failed} need attention.`
        : `Synced ${result.pushed} sighting${result.pushed === 1 ? "" : "s"}.`;

    case false:
      switch (result.reason) {
        case "offline":
          return "Can't sync — offline.";
        case "no-auth":
          return "Please log in to sync.";
        case "busy":
          return "Sync already in progress.";
        case "network":
        default:
          return "Network error. Retrying later.";
      }
  }
}

export function SyncStatusBar() {
  const online = useOnline();
  const { syncing, lastResult } = useSyncState();

  const pendingCountQuerier = () =>
    db.sightings.where("syncStatus").equals("pending").count();
  const pending = useLiveQuery(pendingCountQuerier);
  const failedCountQuerier = () =>
    db.sightings.where("syncStatus").equals("failed").count();
  const failed = useLiveQuery(failedCountQuerier);

  const summary = syncing ? "Syncing..." : describeResult(lastResult);

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 bg-white px-4 py-2 text-sm">
      <div
        role="status"
        aria-live="polite"
        className="flex flex-wrap items-center gap-2"
      >
        <span
          aria-hidden="true"
          className={`h-2.5 w-2.5 rounded-full ${online ? "bg-success" : "bg-danger"}`}
        />
        <span className="font-medium text-ink">
          {online ? "Online" : "Offline"}
        </span>
        {pending !== undefined && pending > 0 && (
          <span className="text-muted">{pending} pending</span>
        )}
        {failed !== undefined && failed > 0 && (
          <span className="text-danger">{failed} failed</span>
        )}
        <span className="text-muted">{summary}</span>
      </div>
      <Button
        type="button"
        onClick={() => void syncNow()}
        disabled={syncing}
        className="h-12"
      >
        Sync now
      </Button>
    </div>
  );
}
