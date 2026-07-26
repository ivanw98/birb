import { newSightingID } from "./id";
import { deviceOffsetMinutes, nowISO } from "./time";
import { getFix } from "./geo";
import type { LocalSighting } from "@/types";

export async function captureSighting(): Promise<LocalSighting> {
  const observedAt = nowISO();
  const base: LocalSighting = {
    id: newSightingID(),
    observedAt,
    observedAtOffsetMinutes: deviceOffsetMinutes(),
    clientUpdatedAt: observedAt,
    photoPaths: [],
    syncStatus: "pending",
  };

  const fix = await getFix();
  return fix ? { ...base, ...fix } : base;
}
