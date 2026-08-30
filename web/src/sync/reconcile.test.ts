import { describe, expect, it } from "vitest";
import { reconcileItem } from "./syncEngine";
import type { LocalSighting } from "../types";
import type { components } from "../api/schema";

type BatchItemResult = components["schemas"]["BatchItemResult"];

function makeSighting(overrides: Partial<LocalSighting> = {}): LocalSighting {
  return {
    id: "sgh_00000000000000000000000001",
    observedAt: "2026-06-01T09:00:00.000Z",
    observedAtOffsetMinutes: 60,
    clientUpdatedAt: "2026-06-01T09:00:00.000Z",
    photoPaths: [],
    recordingPaths: [],
    syncStatus: "pending",
    ...overrides,
  };
}

describe("reconcileItem", () => {
  it.each(["created", "updated", "stale"] as const)(
    "marks the row synced and clears syncError on a %s result",
    (status) => {
      const sent = makeSighting();
      const current = makeSighting({ syncError: "some-previous-failure" });
      const result: BatchItemResult = { id: sent.id, status };

      expect(reconcileItem(sent, current, result)).toEqual({
        kind: "patch",
        patch: {
          syncStatus: "synced",
          syncError: undefined,
        },
      });
    },
  );

  it("marks the row failed with the server's error slug on an invalid result", () => {
    const sent = makeSighting();
    const current = makeSighting();
    const result: BatchItemResult = {
      id: sent.id,
      status: "invalid",
      error: { code: "unknown_bird_id", message: "no bird with that id" },
    };

    expect(reconcileItem(sent, current, result)).toEqual({
      kind: "patch",
      patch: {
        syncStatus: "failed",
        syncError: "unknown_bird_id",
      },
    });
  });

  it("falls back to a generic slug if an invalid result omits the error body", () => {
    const sent = makeSighting();
    const current = makeSighting();
    const result: BatchItemResult = { id: sent.id, status: "invalid" };

    expect(reconcileItem(sent, current, result)).toEqual({
      kind: "patch",
      patch: {
        syncStatus: "failed",
        syncError: "invalid",
      },
    });
  });

  it("returns null when the row was edited again while the batch was in flight", () => {
    const sent = makeSighting({ clientUpdatedAt: "2026-06-01T09:00:00.000Z" });
    const current = makeSighting({
      clientUpdatedAt: "2026-06-01T09:05:00.000Z",
    });
    const result: BatchItemResult = { id: sent.id, status: "updated" };

    expect(reconcileItem(sent, current, result)).toBeNull();
  });

  it("returns null rather than throwing if the row can no longer be found locally", () => {
    const sent = makeSighting();
    const result: BatchItemResult = { id: sent.id, status: "updated" };

    expect(reconcileItem(sent, undefined, result)).toBeNull();
  });
});
