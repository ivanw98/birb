import { useEffect, useState } from "react";
import { StatusBanner } from "./StatusBanner";

// Chromium only (non-standard)
interface BeforeInstallPromptEvent extends Event {
  readonly platform: string;
  readonly userChoice: Promise<{
    outcome: "accepted" | "dismissed";
    platform: string;
  }>;
  prompt(): Promise<void>;
}

function isStandalone(): boolean {
  const iosNav = navigator as Navigator & { standalone?: boolean };

  // iOS safari never reports display-mode reliably, hence the 2nd check:
  return (
    window.matchMedia("(display-mode: standalone)").matches ||
    iosNav.standalone === true
  );
}
export function InstallPrompt() {
  const [defer, setDefer] = useState<BeforeInstallPromptEvent | null>(null);
  const [installed, setInstalled] = useState<boolean>(isStandalone());

  useEffect(() => {
    const onBeforeInstallPrompt = (e: Event) => {
      e.preventDefault();
      setDefer(e as BeforeInstallPromptEvent);
    };

    const onInstalled = () => {
      setInstalled(true);
      setDefer(null);
    };

    window.addEventListener("beforeinstallprompt", onBeforeInstallPrompt);
    window.addEventListener("appinstalled", onInstalled);
    return () => {
      window.removeEventListener("beforeinstallprompt", onBeforeInstallPrompt);
      window.removeEventListener("appinstalled", onInstalled);
    };
  }, []);

  if (installed) return null;

  if (defer) {
    return (
      <StatusBanner tone="info">
        <span>Install birb for quicker access and offline-safe storage.</span>
        <button
          type="button"
          className="ml-3 h-12 rounded-md bg-primary px-4 text-white"
          onClick={() => {
            void defer.prompt();
            void defer.userChoice.finally(() => setDefer(null));
          }}
        >
          Install
        </button>
      </StatusBanner>
    );
  }

  if (/iphone|ipad|ipod/i.test(navigator.userAgent)) {
    return (
      <StatusBanner tone="info">
        Add birb to your Home Screen: tap Share, then "Add to Home Screen".
      </StatusBanner>
    );
  }

  return null;
}
