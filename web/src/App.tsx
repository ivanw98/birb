import { useEffect, useState } from "react";
import { useLiveQuery } from "dexie-react-hooks";
import { AppShell } from "./components/AppShell";
import { DevResetButton } from "./components/DevResetButton";
import { OfflineBanner } from "./components/OfflineBanner";
import { RecordView } from "./components/RecordView";
import { SightingsView } from "./components/SightingsView";
import { SyncStatusBar } from "./components/SyncStatusBar";
import { useCapture } from "./hooks/useCapture";
import { useDocumentTitle } from "./hooks/useDocumentTitle";
import { useMe } from "./hooks/useMe";
import { refreshBirds } from "./api/birds";
import { db } from "./db/db";
import { gcSyncedTombstones, syncNow } from "./sync/syncEngine";
import { InstallPrompt } from "./components/InstallPrompt";
import { ReloadPrompt } from "./components/ReloadPrompt";
import { useAuth } from "./auth/AuthContext";
import { Button } from "./components/ui/button";
import { SignInScreen } from "./components/SignInScreen";

export type View = "record" | "sightings";

function HeaderIdentity() {
  const me = useMe();
  if (me.status !== "success") return null;

  return (
    <span className="text-sm text-muted">
      {me.data.displayName ?? me.data.email}
    </span>
  );
}

export default function App() {
  const [view, setView] = useState<View>("record");
  const { status, session, signOut } = useAuth();
  const { record, busy, last, saveError, bannerDismissed, dismissBanner } =
    useCapture();

  // refreshBirds is completely static and lives entirely outside the React component lifecycle, hence no deps
  // run's on startup
  useEffect(() => {
    // keep the offline species cache warm
    void refreshBirds().catch(() => {});
    // sweep synced tombstones: no Undo banner can exist yet on a fresh load
    void gcSyncedTombstones();
    // app start
    void syncNow();
    // on connectivity being restored
    const onOnline = () => void syncNow();
    window.addEventListener("online", onOnline);
    return () => window.removeEventListener("online", onOnline);
  }, []);

  const querier = () =>
    db.sightings.where("syncStatus").equals("pending").count();
  const pendingCount = useLiveQuery(querier) ?? 0;

  useDocumentTitle(pendingCount > 0 ? `(${pendingCount}) birb` : "birb");

  if (status === "loading") {
    return (
      <div className="flex min-h-screen items-center justify-center bg-white">
        <p className="text-lg text-muted">Loading birb…</p>
      </div>
    );
  }

  if (status === "signedOut") {
    return <SignInScreen />;
  }

  return (
    <AppShell view={view} onNavigate={setView} headerRight={<HeaderIdentity />}>
      <div className="flex items-center justify-between gap-4 border-b border-slate-300 p-3">
        <span className="text-base text-muted">
          Signed in as {session?.email}
        </span>
        <Button
          type="button"
          variant="outline"
          className="h-12"
          onClick={() => void signOut()}
        >
          Sign out
        </Button>
      </div>
      <SyncStatusBar />
      <OfflineBanner />
      <DevResetButton />
      {view === "record" ? (
        <RecordView
          record={record}
          busy={busy}
          last={last}
          saveError={saveError}
          bannerDismissed={bannerDismissed}
          onDismissBanner={dismissBanner}
        />
      ) : (
        <SightingsView />
      )}
      <InstallPrompt />
      <ReloadPrompt />
    </AppShell>
  );
}
