import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Mock } from "vitest";
import { db } from "@/db/db";
import type { LocalSighting } from "@/types";
import type { components } from "@/api/schema";
import type * as SyncEngineModule from "@/sync/syncEngine";

vi.mock("@/api/client", () => ({ apiClient: { PUT: vi.fn() } }));
vi.mock("@/sync/syncEngine", async (importOriginal) => {
  const actual = await importOriginal<typeof SyncEngineModule>();
  return {
    ...actual,
    syncNow: vi.fn().mockResolvedValue({ ok: true, pushed: 0, failed: 0 }),
    syncSettled: vi.fn().mockResolvedValue(undefined),
  };
});

import { apiClient } from "@/api/client";
import { syncNow, syncSettled } from "@/sync/syncEngine";
import {
  deleteSighting,
  removePhoto,
  removeRecording,
  saveEnrichment,
  undoDelete,
} from "./enrich";

type Sighting = components["schemas"]["Sighting"];
type StaleUpdate = components["schemas"]["StaleUpdate"];

interface PutBody {
  clientUpdatedAt: string;
  birdId?: string;
  quickNote?: string;
  notes?: string;
  photoPaths: string[];
  recordingPaths: string[];
}

interface PutInit {
  params: { path: { id: string } };
  body: PutBody;
}

interface MockPutResult {
  data?: Sighting;
  error?: StaleUpdate;
  response: Response;
}

type PutCall = (path: string, init: PutInit) => Promise<MockPutResult>;

const put = apiClient.PUT as unknown as Mock<PutCall>;

function setOnline(value: boolean) {
  vi.spyOn(navigator, "onLine", "get").mockReturnValue(value);
}

function makeSighting(overrides: Partial<LocalSighting> = {}): LocalSighting {
  return {
    id: "sgh_00000000000000000000000001",
    observedAt: "2026-06-01T09:00:00.000Z",
    observedAtOffsetMinutes: 0,
    clientUpdatedAt: "2026-06-01T09:00:00.000Z",
    photoPaths: [],
    recordingPaths: [],
    syncStatus: "synced",
    ...overrides,
  };
}

function makeRemote(overrides: Partial<Sighting> = {}): Sighting {
  return {
    id: "sgh_00000000000000000000000001",
    observedAt: "2026-06-01T09:00:00.000Z",
    observedAtOffsetMinutes: 0,
    clientUpdatedAt: "2026-06-01T09:05:00.000Z",
    createdAt: "2026-06-01T09:00:00.000Z",
    updatedAt: "2026-06-01T09:05:00.000Z",
    photoPaths: [],
    recordingPaths: [],
    ...overrides,
  };
}

function putResult(overrides: Partial<MockPutResult>): MockPutResult {
  return {
    data: undefined,
    error: undefined,
    response: new Response(null, { status: 200 }),
    ...overrides,
  };
}

beforeEach(async () => {
  await db.sightings.clear();
  await db.photos.clear();
  await db.recordings.clear();
  setOnline(true);
});

afterEach(() => {
  vi.clearAllMocks();
});

describe("removePhoto", () => {
  it("deletes a queued photo locally without calling the API", async () => {
    const id = await db.photos.add({
      sightingId: "sgh_00000000000000000000000001",
      fileName: "a.jpg",
      blob: new Blob(),
      uploaded: 0,
    });

    const result = await removePhoto(makeSighting(), { kind: "queued", id });

    expect(result).toEqual({ outcome: "removed" });
    expect(await db.photos.get(id)).toBeUndefined();
    expect(put).not.toHaveBeenCalled();
  });

  it("returns offline when there is no connection", async () => {
    setOnline(false);

    const result = await removePhoto(makeSighting(), {
      kind: "attached",
      path: "auth1/sgh_00000000000000000000000001/a.jpg",
    });

    expect(result).toEqual({ outcome: "offline" });
    expect(put).not.toHaveBeenCalled();
  });

  it("returns offline when the row has not finished its initial sync", async () => {
    const result = await removePhoto(
      makeSighting({ syncStatus: "pending" }),
      { kind: "attached", path: "auth1/sgh_00000000000000000000000001/a.jpg" },
    );

    expect(result).toEqual({ outcome: "offline" });
    expect(put).not.toHaveBeenCalled();
  });

  it("removes an attached photo and reclaims its local blob", async () => {
    const row = makeSighting({
      photoPaths: [
        "auth1/sgh_00000000000000000000000001/a.jpg",
        "auth1/sgh_00000000000000000000000001/b.jpg",
      ],
    });
    await db.photos.add({
      sightingId: row.id,
      fileName: "a.jpg",
      blob: new Blob(),
      uploaded: 1,
    });
    put.mockResolvedValueOnce(
      putResult({ data: makeRemote({ photoPaths: [row.photoPaths[1]] }) }),
    );

    const result = await removePhoto(row, {
      kind: "attached",
      path: row.photoPaths[0],
    });

    expect(result).toEqual({ outcome: "removed" });
    expect(put.mock.calls[0][1].body.photoPaths).toEqual([row.photoPaths[1]]);
    expect(await db.photos.where("sightingId").equals(row.id).count()).toBe(
      0,
    );
  });

  it("returns conflict and stores the current row on a 409", async () => {
    const row = makeSighting({
      photoPaths: ["auth1/sgh_00000000000000000000000001/a.jpg"],
    });
    await db.sightings.add(row);
    const current = makeRemote({ quickNote: "someone else's edit" });
    put.mockResolvedValueOnce(
      putResult({
        error: { code: "stale_update", current },
        response: new Response(null, { status: 409 }),
      }),
    );

    const result = await removePhoto(row, {
      kind: "attached",
      path: row.photoPaths[0],
    });

    expect(result).toEqual({ outcome: "conflict" });
    expect((await db.sightings.get(row.id))?.quickNote).toBe(
      "someone else's edit",
    );
  });

  it("adopts a remote tombstone and returns gone on a 404", async () => {
    const row = makeSighting({
      photoPaths: ["auth1/sgh_00000000000000000000000001/a.jpg"],
    });
    await db.sightings.add(row);
    put.mockResolvedValueOnce(
      putResult({ response: new Response(null, { status: 404 }) }),
    );

    const result = await removePhoto(row, {
      kind: "attached",
      path: row.photoPaths[0],
    });

    expect(result).toEqual({ outcome: "gone" });
    expect(await db.sightings.get(row.id)).toMatchObject({
      deleted: 1,
      syncStatus: "synced",
    });
  });

  it("returns an error outcome for an unexpected status", async () => {
    const row = makeSighting({
      photoPaths: ["auth1/sgh_00000000000000000000000001/a.jpg"],
    });
    put.mockResolvedValueOnce(
      putResult({ response: new Response(null, { status: 500 }) }),
    );

    const result = await removePhoto(row, {
      kind: "attached",
      path: row.photoPaths[0],
    });

    expect(result).toEqual({
      outcome: "error",
      message: "remove failed: 500",
    });
  });

  it("returns offline when the request throws", async () => {
    const row = makeSighting({
      photoPaths: ["auth1/sgh_00000000000000000000000001/a.jpg"],
    });
    put.mockRejectedValueOnce(new Error("network down"));

    const result = await removePhoto(row, {
      kind: "attached",
      path: row.photoPaths[0],
    });

    expect(result).toEqual({ outcome: "offline" });
  });
});

describe("removeRecording", () => {
  it("deletes a queued recording locally, not from db.photos", async () => {
    const id = await db.recordings.add({
      sightingId: "sgh_00000000000000000000000001",
      fileName: "a.webm",
      blob: new Blob(),
      mimeType: "audio/webm",
      uploaded: 0,
    });

    const result = await removeRecording(makeSighting(), {
      kind: "queued",
      id,
    });

    expect(result).toEqual({ outcome: "removed" });
    expect(await db.recordings.get(id)).toBeUndefined();
  });

  it("removes an attached recording, leaving photoPaths untouched", async () => {
    const row = makeSighting({
      photoPaths: ["auth1/sgh_00000000000000000000000001/a.jpg"],
      recordingPaths: ["auth1/sgh_00000000000000000000000001/a.webm"],
    });
    await db.recordings.add({
      sightingId: row.id,
      fileName: "a.webm",
      blob: new Blob(),
      mimeType: "audio/webm",
      uploaded: 1,
    });
    put.mockResolvedValueOnce(
      putResult({ data: makeRemote({ photoPaths: row.photoPaths }) }),
    );

    const result = await removeRecording(row, {
      kind: "attached",
      path: row.recordingPaths[0],
    });

    expect(result).toEqual({ outcome: "removed" });
    expect(put.mock.calls[0][1].body.recordingPaths).toEqual([]);
    expect(put.mock.calls[0][1].body.photoPaths).toEqual(row.photoPaths);
    expect(
      await db.recordings.where("sightingId").equals(row.id).count(),
    ).toBe(0);
  });

  it("adopts a remote tombstone and returns gone on a 404", async () => {
    const row = makeSighting({
      recordingPaths: ["auth1/sgh_00000000000000000000000001/a.webm"],
    });
    await db.sightings.add(row);
    put.mockResolvedValueOnce(
      putResult({ response: new Response(null, { status: 404 }) }),
    );

    const result = await removeRecording(row, {
      kind: "attached",
      path: row.recordingPaths[0],
    });

    expect(result).toEqual({ outcome: "gone" });
    expect(await db.sightings.get(row.id)).toMatchObject({ deleted: 1 });
  });

  it("returns offline when there is no connection", async () => {
    setOnline(false);

    const result = await removeRecording(makeSighting(), {
      kind: "attached",
      path: "auth1/sgh_00000000000000000000000001/a.webm",
    });

    expect(result).toEqual({ outcome: "offline" });
    expect(put).not.toHaveBeenCalled();
  });
});

describe("deleteSighting", () => {
  it("returns false and does not sync when the row does not exist", async () => {
    const result = await deleteSighting("sgh_missing000000000000000001");

    expect(result).toBe(false);
    expect(syncNow).not.toHaveBeenCalled();
  });

  it("returns false when the row is already deleted", async () => {
    const row = makeSighting({ deleted: 1 });
    await db.sightings.add(row);

    const result = await deleteSighting(row.id);

    expect(result).toBe(false);
    expect(syncNow).not.toHaveBeenCalled();
  });

  it("tombstones the row, bumps clientUpdatedAt, and triggers a sync", async () => {
    const row = makeSighting({ syncError: "some-previous-failure" });
    await db.sightings.add(row);

    const result = await deleteSighting(row.id);

    expect(result).toBe(true);
    const updated = await db.sightings.get(row.id);
    expect(updated).toMatchObject({ deleted: 1, syncStatus: "pending" });
    expect(updated?.syncError).toBeUndefined();
    expect(Date.parse(updated!.clientUpdatedAt)).toBeGreaterThan(
      Date.parse(row.clientUpdatedAt),
    );
    expect(syncSettled).toHaveBeenCalled();
    expect(syncNow).toHaveBeenCalledTimes(1);
  });
});

describe("undoDelete", () => {
  it("returns false when the row is not deleted", async () => {
    const row = makeSighting({ deleted: 0 });
    await db.sightings.add(row);

    const result = await undoDelete(row.id);

    expect(result).toBe(false);
    expect(syncNow).not.toHaveBeenCalled();
  });

  it("returns false when the row does not exist", async () => {
    const result = await undoDelete("sgh_missing000000000000000001");

    expect(result).toBe(false);
    expect(syncNow).not.toHaveBeenCalled();
  });

  it("restores a deleted row, bumps clientUpdatedAt, and triggers a sync", async () => {
    const row = makeSighting({ deleted: 1 });
    await db.sightings.add(row);

    const result = await undoDelete(row.id);

    expect(result).toBe(true);
    const updated = await db.sightings.get(row.id);
    expect(updated).toMatchObject({ deleted: 0, syncStatus: "pending" });
    expect(Date.parse(updated!.clientUpdatedAt)).toBeGreaterThan(
      Date.parse(row.clientUpdatedAt),
    );
    expect(syncNow).toHaveBeenCalledTimes(1);
  });
});

describe("saveEnrichment", () => {
  const content = {
    birdId: "brd_00000000000000000000000001",
    quickNote: "wren",
    notes: "",
  };

  it("saves locally without calling the API when offline", async () => {
    setOnline(false);
    const row = makeSighting();
    await db.sightings.add(row);

    const result = await saveEnrichment(row, content);

    expect(result).toEqual({ outcome: "saved-offline" });
    expect(put).not.toHaveBeenCalled();
    expect(await db.sightings.get(row.id)).toMatchObject({
      birdId: content.birdId,
      quickNote: content.quickNote,
      syncStatus: "pending",
    });
  });

  it("saves locally when the row has never finished its initial sync", async () => {
    const row = makeSighting({ syncStatus: "pending" });
    await db.sightings.add(row);

    const result = await saveEnrichment(row, content);

    expect(result).toEqual({ outcome: "saved-offline" });
    expect(put).not.toHaveBeenCalled();
  });

  it("saves online and stores the server's row on success", async () => {
    const row = makeSighting();
    await db.sightings.add(row);
    const remote = makeRemote({ quickNote: content.quickNote });
    put.mockResolvedValueOnce(putResult({ data: remote }));

    const result = await saveEnrichment(row, content);

    expect(result).toEqual({ outcome: "saved-online" });
    expect(put.mock.calls[0][1].body).toEqual({
      clientUpdatedAt: expect.any(String),
      ...content,
      photoPaths: row.photoPaths,
      recordingPaths: row.recordingPaths,
    });
    expect(await db.sightings.get(row.id)).toMatchObject({
      quickNote: content.quickNote,
      syncStatus: "synced",
    });
  });

  it("returns conflict with the current row on a 409", async () => {
    const row = makeSighting();
    await db.sightings.add(row);
    const current = makeRemote({ quickNote: "someone else's edit" });
    put.mockResolvedValueOnce(
      putResult({
        error: { code: "stale_update", current },
        response: new Response(null, { status: 409 }),
      }),
    );

    const result = await saveEnrichment(row, content);

    expect(result).toEqual({ outcome: "conflict", current });
    expect((await db.sightings.get(row.id))?.quickNote).toBe(
      "someone else's edit",
    );
  });

  it("adopts a remote tombstone and returns gone on a 404", async () => {
    const row = makeSighting();
    await db.sightings.add(row);
    put.mockResolvedValueOnce(
      putResult({ response: new Response(null, { status: 404 }) }),
    );

    const result = await saveEnrichment(row, content);

    expect(result).toEqual({ outcome: "gone" });
    expect(await db.sightings.get(row.id)).toMatchObject({ deleted: 1 });
  });

  it("returns an error outcome for an unexpected status", async () => {
    const row = makeSighting();
    await db.sightings.add(row);
    put.mockResolvedValueOnce(
      putResult({ response: new Response(null, { status: 500 }) }),
    );

    const result = await saveEnrichment(row, content);

    expect(result).toEqual({ outcome: "error", message: "save failed: 500" });
  });

  it("falls back to saving locally when the request throws", async () => {
    const row = makeSighting();
    await db.sightings.add(row);
    put.mockRejectedValueOnce(new Error("network down"));

    const result = await saveEnrichment(row, content);

    expect(result).toEqual({ outcome: "saved-offline" });
    expect(await db.sightings.get(row.id)).toMatchObject({
      quickNote: content.quickNote,
      syncStatus: "pending",
    });
  });
});
