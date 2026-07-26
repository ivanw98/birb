import { RecordButton } from "./RecordButton";
import { StatusBanner } from "./StatusBanner";
import { QuickTag } from "./QuickTag";
import { useGeoPermission } from "@/hooks/useGeoPermission";
import type { LocalSighting } from "@/types";

type CaptureBanner = { tone: "info" | "success"; message: string };

// Tone and message are one decision, so derive them together — as two
// separate ternaries in the JSX they could drift apart.
function captureBannerFor(last: LocalSighting | null): CaptureBanner | null {
  if (!last) return null;
  if (last.latitude === undefined) {
    return {
      tone: "info",
      message:
        "Saved without location — turn on location access to add coordinates next time.",
    };
  }
  return { tone: "success", message: "Sighting saved with location." };
}

export interface RecordViewProps {
  record: () => Promise<void>;
  busy: boolean;
  last: LocalSighting | null;
  saveError: string | null;
}

// Capture state is owned by App, not here. This view unmounts on every
// navigation, and state owned locally would take the capture banner and the
// QuickTag surface with it each time you glance at the list and come back.
// useGeoPermission stays local: nothing outside this view reads it.
export function RecordView({ record, busy, last, saveError }: RecordViewProps) {
  const permission = useGeoPermission();
  const banner = captureBannerFor(last);

  return (
    <div className="flex flex-col items-center gap-4 p-6">
      {permission === "denied" && (
        <StatusBanner tone="info">
          Location access is turned off. Sightings will still save, without
          coordinates, until you turn it back on for this site.
        </StatusBanner>
      )}
      <h2 className="text-2xl font-semibold text-ink">Record a Sighting</h2>
      <RecordButton onRecord={record} busy={busy} />
      {banner && <StatusBanner tone={banner.tone}>{banner.message}</StatusBanner>}
      {/* lesson 7b: identity travels with the moment. `last` lives here, so
          this is the mount point — QuickTag keys itself on the row id. */}
      {last && <QuickTag sightingId={last.id} />}
      {saveError && (
        <StatusBanner tone="danger">
          Could not save this sighting — your device may be low on storage.
          Please try again.
        </StatusBanner>
      )}
    </div>
  );
}
