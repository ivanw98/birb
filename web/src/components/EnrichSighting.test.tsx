import { afterEach, describe, expect, it, vi } from "vitest";
import type { Mock } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { db, type LocalPhoto, type LocalRecording } from "@/db/db";
import type { LocalSighting } from "@/types";
import type { components } from "@/api/schema";
import type * as SyncEngineModule from "@/sync/syncEngine";
import { EnrichSighting } from "./EnrichSighting";

vi.mock("./RecordingPlayer", () => ({
  RecordingPlayer: ({ path }: { path?: string; blob?: Blob }) => (
    <span>{path ? `player for ${path}` : "player for a queued recording"}</span>
  ),
}));
vi.mock("./PhotoThumb", () => ({
  PhotoThumb: ({ path }: { path?: string; blob?: Blob }) => (
    <span>{path ? `thumb for ${path}` : "thumb for a queued photo"}</span>
  ),
}));
vi.mock("@/api/client", () => ({ apiClient: { PUT: vi.fn() } }));
vi.mock("@/sync/syncEngine", async (importOriginal) => {
  const actual = await importOriginal<typeof SyncEngineModule>();
  return { ...actual, syncSettled: vi.fn().mockResolvedValue(undefined) };
});

import { apiClient } from "@/api/client";
import { syncSettled } from "@/sync/syncEngine";

type Sighting = components["schemas"]["Sighting"];

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
  response: Response;
}
type PutCall = (path: string, init: PutInit) => Promise<MockPutResult>;

const put = apiClient.PUT as unknown as Mock<PutCall>;
const settled = vi.mocked(syncSettled);

function sightingId(suffix: string): string {
  return `sgh_${suffix.padEnd(26, "0")}`;
}

const ID = sightingId("enrich");

function sighting(overrides: Partial<LocalSighting> = {}): LocalSighting {
  return {
    id: ID,
    observedAt: "2026-08-30T09:00:00.000Z",
    observedAtOffsetMinutes: 600,
    clientUpdatedAt: "2026-08-30T09:00:00.000Z",
    photoPaths: [],
    recordingPaths: [],
    syncStatus: "synced",
    ...overrides,
  };
}

function photo(overrides: Partial<LocalPhoto> = {}): LocalPhoto {
  return {
    sightingId: ID,
    fileName: "clip.jpg",
    blob: new Blob(["img"], { type: "image/jpeg" }),
    uploaded: 0,
    ...overrides,
  };
}

function recording(overrides: Partial<LocalRecording> = {}): LocalRecording {
  return {
    sightingId: ID,
    fileName: "clip.webm",
    blob: new Blob(["audio"], { type: "audio/webm" }),
    mimeType: "audio/webm",
    uploaded: 0,
    ...overrides,
  };
}

function makeRemote(overrides: Partial<Sighting> = {}): Sighting {
  return {
    id: ID,
    observedAt: "2026-08-30T09:00:00.000Z",
    observedAtOffsetMinutes: 600,
    clientUpdatedAt: "2026-08-30T09:05:00.000Z",
    createdAt: "2026-08-30T09:00:00.000Z",
    updatedAt: "2026-08-30T09:05:00.000Z",
    photoPaths: [],
    recordingPaths: [],
    ...overrides,
  };
}

interface Spy {
  mockReturnValue: (v: unknown) => void;
}

function pendingArray<T>() {
  const { promise, resolve } = Promise.withResolvers<T[]>();
  return { promise, resolve };
}

describe("EnrichSighting", () => {
  afterEach(async () => {
    vi.restoreAllMocks();
    settled.mockResolvedValue(undefined);
    await db.sightings.clear();
    await db.photos.clear();
    await db.recordings.clear();
  });

  describe("queued-media loading", () => {
    it("shows a loading placeholder for photos until the queued-photo query resolves", async () => {
      await db.sightings.add(sighting());
      const { promise, resolve } = pendingArray<LocalPhoto>();
      (vi.spyOn(db.photos, "where") as unknown as Spy).mockReturnValue({
        equals: () => ({ and: () => ({ toArray: () => promise }) }),
      });

      render(
        <EnrichSighting sightingId={ID} onClose={() => {}} onDeleted={() => {}} />,
      );

      expect(await screen.findByText("Loading photos…")).toBeInTheDocument();
      expect(screen.queryByLabelText(/Add a photo/)).not.toBeInTheDocument();

      resolve([]);

      expect(await screen.findByLabelText(/Add a photo/)).toBeInTheDocument();
      expect(screen.queryByText("Loading photos…")).not.toBeInTheDocument();
    });

    it("shows a loading placeholder for recordings until the queued-recording query resolves", async () => {
      await db.sightings.add(sighting());
      const { promise, resolve } = pendingArray<LocalRecording>();
      (vi.spyOn(db.recordings, "where") as unknown as Spy).mockReturnValue({
        equals: () => ({ and: () => ({ toArray: () => promise }) }),
      });

      render(
        <EnrichSighting sightingId={ID} onClose={() => {}} onDeleted={() => {}} />,
      );

      expect(
        await screen.findByText("Loading recordings…"),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Start recording" }),
      ).not.toBeInTheDocument();

      resolve([]);

      expect(
        await screen.findByRole("button", { name: "Start recording" }),
      ).toBeInTheDocument();
      expect(
        screen.queryByText("Loading recordings…"),
      ).not.toBeInTheDocument();
    });
  });

  describe("media allowance", () => {
    it("stops offering new photos once the sighting holds ten", async () => {
      await db.sightings.add(
        sighting({
          photoPaths: Array.from({ length: 9 }, (_, i) => `uid/sgh/${i}.jpg`),
        }),
      );
      await db.photos.add(photo({ fileName: "tenth.jpg" }));

      render(
        <EnrichSighting sightingId={ID} onClose={() => {}} onDeleted={() => {}} />,
      );

      expect(
        await screen.findByText(
          "This sighting already has 10 photos, the most the app keeps per sighting.",
        ),
      ).toBeInTheDocument();
      expect(screen.getByLabelText(/Add a photo/)).toBeDisabled();
    });

    it("stops offering new recordings once the sighting holds five", async () => {
      await db.sightings.add(
        sighting({
          recordingPaths: ["a", "b", "c", "d"].map(
            (n) => `uid/sgh/${n}.webm`,
          ),
        }),
      );
      await db.recordings.add(recording({ fileName: "fifth.webm" }));

      render(
        <EnrichSighting sightingId={ID} onClose={() => {}} onDeleted={() => {}} />,
      );

      expect(
        await screen.findByText(
          "This sighting already has 5 recordings, the most the app keeps per sighting.",
        ),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Start recording" }),
      ).toBeDisabled();
    });
  });

  describe("save during an in-flight sync", () => {
    it("waits out the sync pass, then saves the row's freshest state, not the one captured at render", async () => {
      await db.sightings.add(sighting());
      put.mockResolvedValueOnce({
        data: makeRemote({ photoPaths: ["uid/sgh/synced-elsewhere.jpg"] }),
        response: new Response(null, { status: 200 }),
      });
      const { promise: syncPromise, resolve: resolveSync } =
        Promise.withResolvers<void>();
      settled.mockReturnValueOnce(syncPromise);

      const onClose = vi.fn();
      render(
        <EnrichSighting sightingId={ID} onClose={onClose} onDeleted={() => {}} />,
      );
      await screen.findByLabelText(/Add a photo/);

      await userEvent.click(screen.getByRole("button", { name: "Save" }));
      expect(
        await screen.findByRole("button", { name: "Saving…" }),
      ).toBeInTheDocument();
      expect(put).not.toHaveBeenCalled();

      // The sync pass handleSubmit is waiting on lands its own PUT while we're
      // paused, attaching a photo that `row` (captured at render) never saw.
      await db.sightings.update(ID, {
        photoPaths: ["uid/sgh/synced-elsewhere.jpg"],
        clientUpdatedAt: "2026-08-30T09:05:00.000Z",
      });
      resolveSync();

      await waitFor(() => expect(onClose).toHaveBeenCalled());
      expect(put.mock.calls[0][1].body.photoPaths).toEqual([
        "uid/sgh/synced-elsewhere.jpg",
      ]);
    });
  });

  describe("cross-guarding Save and Remove", () => {
    it("disables Save while a photo removal is in flight", async () => {
      await db.sightings.add(
        sighting({ photoPaths: ["uid/sgh/a.jpg"] }),
      );
      const { promise: removePromise, resolve: resolveRemove } =
        Promise.withResolvers<MockPutResult>();
      put.mockReturnValueOnce(removePromise);

      render(
        <EnrichSighting sightingId={ID} onClose={() => {}} onDeleted={() => {}} />,
      );
      await screen.findByLabelText(/Add a photo/);

      const removeButton = screen.getByRole("button", {
        name: /Remove photo/,
      });
      await userEvent.click(removeButton);
      await waitFor(() =>
        expect(removeButton).toHaveTextContent("Removing…"),
      );
      expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();

      resolveRemove({
        data: makeRemote({ photoPaths: [] }),
        response: new Response(null, { status: 200 }),
      });

      await waitFor(() =>
        expect(screen.getByRole("button", { name: "Save" })).not.toBeDisabled(),
      );
    });

    it("disables Remove while a save is in flight", async () => {
      await db.sightings.add(
        sighting({ photoPaths: ["uid/sgh/a.jpg"] }),
      );
      const { promise: savePromise, resolve: resolveSave } =
        Promise.withResolvers<MockPutResult>();
      put.mockReturnValueOnce(savePromise);

      render(
        <EnrichSighting sightingId={ID} onClose={() => {}} onDeleted={() => {}} />,
      );
      await screen.findByLabelText(/Add a photo/);

      await userEvent.click(screen.getByRole("button", { name: "Save" }));
      expect(
        await screen.findByRole("button", { name: "Saving…" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: /Remove photo/ }),
      ).toBeDisabled();

      resolveSave({
        data: makeRemote({ photoPaths: ["uid/sgh/a.jpg"] }),
        response: new Response(null, { status: 200 }),
      });

      await waitFor(() =>
        expect(
          screen.getByRole("button", { name: /Remove photo/ }),
        ).not.toBeDisabled(),
      );
    });
  });
});
