import { db } from "@/db/db";
import type { LocalSighting } from "@/types";
import { bumpClientUpdatedAt } from "./time";

export async function quickTag(
  row: LocalSighting,
  patch: Partial<Pick<LocalSighting, "birdId" | "quickNote">>,
): Promise<void> {
  await db.sightings.update(row.id, {
    ...patch,
    clientUpdatedAt: bumpClientUpdatedAt(row.clientUpdatedAt),
    syncStatus: "pending",
    syncError: undefined,
  });
}
