import { useRegisterSW } from "virtual:pwa-register/react";
import { StatusBanner } from "./StatusBanner";

export function ReloadPrompt() {
  const {
    offlineReady: [offlineReady, setOfflineReady],
    needRefresh: [needRefresh, setNeedRefresh],
    updateServiceWorker,
  } = useRegisterSW({
    onRegisterError(err) {
      console.error("service worker registration failed", err);
    },
  });

  if (needRefresh) {
    return (
      <StatusBanner tone="info">
        <span>An update to birb is ready.</span>
        <button
          type="button"
          className="ml-3 h-12 rounded-md bg-primary px-4 text-white"
          onClick={() => {
            void updateServiceWorker(true);
          }}
        >
          Reload to update
        </button>
        <button
          type="button"
          className="ml-3 h-12 rounded-md border border-slate-300 bg-white px-4 text-ink"
          onClick={() => setNeedRefresh(false)}
        >
          Later
        </button>
      </StatusBanner>
    );
  }

  if (offlineReady) {
    return (
      <StatusBanner tone="success">
        <span>birb is ready to work offline.</span>
        <button
          type="button"
          className="ml-3 h-12 rounded-md border border-slate-300 bg-white px-4 text-ink"
          onClick={() => setOfflineReady(false)}
        >
          Got it
        </button>
      </StatusBanner>
    );
  }
  return null;
}
