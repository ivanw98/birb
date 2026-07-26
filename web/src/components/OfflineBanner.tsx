import { useOnline } from "@/hooks/useOnline";
import { StatusBanner } from "./StatusBanner";

// Self-guarding: the condition that decides whether this banner exists lives
// here, not in App's JSX. App mounts it unconditionally.
export function OfflineBanner() {
  const online = useOnline();
  if (online) return null;

  return (
    <StatusBanner tone="info">
      You're offline – sightings are saved on this device and will sync once
      you're back online.
    </StatusBanner>
  );
}
