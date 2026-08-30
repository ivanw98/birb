import { db } from "@/db/db";
import { captureSighting } from "@/lib/capture";
import { syncNow } from "@/sync/syncEngine";
import type { LocalSighting } from "@/types";
import { useRef, useState } from "react";

export function useCapture() {
  const [saveError, setSaveError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [last, setLast] = useState<LocalSighting | null>(null);
  // Hides the capture-outcome banner without clearing `last` (which also
  // mounts the details sheet). Owned here so a dismissal survives navigation.
  const [bannerDismissed, setBannerDismissed] = useState(false);
  const [sheetOpen, setSheetOpen] = useState(false);
  const totalAttemptCounter = useRef(0);

  async function record(): Promise<void> {
    setBusy(true);
    setBannerDismissed(false);
    try {
      totalAttemptCounter.current++;
      const sighting = await captureSighting();
      // insert, not upsert
      await db.sightings.add(sighting);

      setLast(sighting);
      setSheetOpen(true);
      // fire-and-forget
      void syncNow();
      setSaveError(null);
    } catch (err) {
      console.error(err);
      setSaveError(err instanceof Error ? err.message : String(err));
      setLast(null);
    } finally {
      setBusy(false);
    }
  }

  return {
    busy,
    last,
    record,
    saveError,
    bannerDismissed,
    dismissBanner: () => setBannerDismissed(true),
    sheetOpen,
    closeSheet: () => setSheetOpen(false),
  };
}
