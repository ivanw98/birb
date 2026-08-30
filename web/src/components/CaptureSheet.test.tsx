import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { db, type LocalRecording } from "../db/db";
import type { LocalSighting } from "../types";
import { CaptureSheet } from "./CaptureSheet";

vi.mock("./RecordingPlayer", () => ({
  RecordingPlayer: ({ path }: { path?: string; blob?: Blob }) => (
    <span>{path ? `player for ${path}` : "player for a queued recording"}</span>
  ),
}));

function sightingId(suffix: string): string {
  return `sgh_${suffix.padEnd(26, "0")}`;
}

const ID = sightingId("capture");

function sighting(overrides: Partial<LocalSighting> = {}): LocalSighting {
  return {
    id: ID,
    observedAt: "2026-08-30T09:00:00.000Z",
    observedAtOffsetMinutes: 600,
    clientUpdatedAt: "2026-08-30T09:00:00.000Z",
    photoPaths: [],
    recordingPaths: [],
    syncStatus: "pending",
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

describe("CaptureSheet", () => {
  afterEach(async () => {
    await db.sightings.clear();
    await db.recordings.clear();
  });

  it("stays closed for an id that isn't in Dexie", async () => {
    render(<CaptureSheet sightingId={ID} open={true} onClose={() => {}} />);

    // A row that never arrives renders identically to one still loading, so the
    // assertion has to wait rather than read the first frame.
    await expect(
      screen.findByRole("dialog", {}, { timeout: 150 }),
    ).rejects.toThrow();
  });

  it("renders nothing while closed even though the row exists", async () => {
    await db.sightings.add(sighting());

    render(<CaptureSheet sightingId={ID} open={false} onClose={() => {}} />);

    await expect(
      screen.findByRole("dialog", {}, { timeout: 150 }),
    ).rejects.toThrow();
  });

  it("disappears when the sighting is tombstoned underneath it", async () => {
    await db.sightings.add(sighting());
    render(<CaptureSheet sightingId={ID} open={true} onClose={() => {}} />);
    await screen.findByRole("dialog");

    await db.sightings.update(ID, { deleted: 1 });

    // A tombstoned row is gone as far as any editor is concerned; edits made
    // against it would be lost when the delete syncs.
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });

  describe("location wording", () => {
    it("says the sighting was saved with a location when one was captured", async () => {
      await db.sightings.add(
        sighting({ latitude: -33.86, longitude: 151.2, accuracyM: 12 }),
      );

      render(<CaptureSheet sightingId={ID} open={true} onClose={() => {}} />);

      expect(
        await screen.findByText(/Saved with your location\./),
      ).toBeInTheDocument();
    });

    it("says the sighting was saved without one when the fix failed", async () => {
      await db.sightings.add(sighting({ latitude: undefined }));

      render(<CaptureSheet sightingId={ID} open={true} onClose={() => {}} />);

      expect(
        await screen.findByText(/Saved without location\./),
      ).toBeInTheDocument();
    });
  });

  describe("recording allowance", () => {
    it("counts uploaded paths and still-queued blobs against the same limit", async () => {
      await db.sightings.add(
        sighting({ recordingPaths: ["uid/sgh/one.webm", "uid/sgh/two.webm"] }),
      );
      await db.recordings.add(recording({ fileName: "three.webm" }));

      render(<CaptureSheet sightingId={ID} open={true} onClose={() => {}} />);

      expect(
        await screen.findByText("Add a recording (2 of 5 left)"),
      ).toBeInTheDocument();
    });

    it("ignores recordings that already uploaded or belong to another sighting", async () => {
      await db.sightings.add(sighting());
      await db.recordings.bulkAdd([
        // Already uploaded: its path is on the sightings row, so counting the
        // queue row too would charge the allowance twice.
        recording({ fileName: "done.webm", uploaded: 1 }),
        recording({
          sightingId: sightingId("other"),
          fileName: "elsewhere.webm",
        }),
        recording({ fileName: "mine.webm" }),
      ]);

      render(<CaptureSheet sightingId={ID} open={true} onClose={() => {}} />);

      expect(
        await screen.findByText("Add a recording (4 of 5 left)"),
      ).toBeInTheDocument();
    });

    it("stops offering new recordings once the sighting holds five", async () => {
      await db.sightings.add(
        sighting({
          recordingPaths: ["a", "b", "c", "d"].map((n) => `uid/sgh/${n}.webm`),
        }),
      );
      await db.recordings.add(recording({ fileName: "fifth.webm" }));

      render(<CaptureSheet sightingId={ID} open={true} onClose={() => {}} />);

      expect(
        await screen.findByRole("button", { name: "Start recording" }),
      ).toBeDisabled();
      expect(
        screen.getByText(
          "This sighting already has 5 recordings, the most the app keeps per sighting.",
        ),
      ).toBeInTheDocument();
    });
  });

  describe("recording list", () => {
    it("renders one player per uploaded path and per queued blob", async () => {
      await db.sightings.add(
        sighting({ recordingPaths: ["uid/sgh/one.webm", "uid/sgh/two.webm"] }),
      );
      await db.recordings.add(recording({ fileName: "three.webm" }));

      render(<CaptureSheet sightingId={ID} open={true} onClose={() => {}} />);

      expect(
        await screen.findByText("player for uid/sgh/one.webm"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("player for uid/sgh/two.webm"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("player for a queued recording"),
      ).toBeInTheDocument();
    });

    it("omits the list entirely when the sighting has no audio", async () => {
      await db.sightings.add(sighting());

      render(<CaptureSheet sightingId={ID} open={true} onClose={() => {}} />);
      await screen.findByRole("dialog");

      expect(screen.queryByRole("list")).not.toBeInTheDocument();
    });

    it("picks up a recording queued while the sheet is open", async () => {
      await db.sightings.add(sighting());
      render(<CaptureSheet sightingId={ID} open={true} onClose={() => {}} />);
      await screen.findByRole("dialog");

      await db.recordings.add(recording());

      expect(
        await screen.findByText("player for a queued recording"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Add a recording (4 of 5 left)"),
      ).toBeInTheDocument();
    });
  });

  describe("closing", () => {
    beforeEach(async () => {
      await db.sightings.add(sighting());
    });

    it("reports a close when Done is pressed", async () => {
      const user = userEvent.setup();
      const onClose = vi.fn();
      render(<CaptureSheet sightingId={ID} open={true} onClose={onClose} />);

      await user.click(await screen.findByRole("button", { name: "Done" }));

      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it("reports a close when the dialog is dismissed with Escape", async () => {
      const user = userEvent.setup();
      const onClose = vi.fn();
      render(<CaptureSheet sightingId={ID} open={true} onClose={onClose} />);
      await screen.findByRole("dialog");

      await user.keyboard("{Escape}");

      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it("reports a close from the corner dismiss button", async () => {
      const user = userEvent.setup();
      const onClose = vi.fn();
      render(<CaptureSheet sightingId={ID} open={true} onClose={onClose} />);

      await user.click(await screen.findByRole("button", { name: "Close" }));

      expect(onClose).toHaveBeenCalledTimes(1);
    });
  });
});
