import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { db } from "../db/db";
import { BirdPicker } from "./BirdPicker";

function birdId(suffix: string): string {
  return `brd_${suffix.padEnd(26, "0")}`;
}

const wren = {
  id: birdId("wren"),
  commonName: "Eurasian Wren",
  scientificName: "Troglodytes troglodytes",
};
const blackbird = {
  id: birdId("blackbird"),
  commonName: "Eurasian Blackbird",
  scientificName: "Turdus merula",
};
const robin = {
  id: birdId("robin"),
  commonName: "European Robin",
  scientificName: "Erithacus rubecula",
};

describe("BirdPicker", () => {
  beforeEach(async () => {
    await db.birds.bulkAdd([wren, blackbird, robin]);
  });

  afterEach(async () => {
    await db.birds.clear();
  });

  it("filters the Dexie bird cache as the user types and reports the chosen id", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<BirdPicker birdId={undefined} onChange={onChange} />);

    await user.type(screen.getByLabelText("Search species"), "wre");

    const wrenButton = await screen.findByRole("button", {
      name: /Eurasian Wren/,
    });
    expect(
      screen.queryByRole("button", { name: /Eurasian Blackbird/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /European Robin/ }),
    ).not.toBeInTheDocument();

    await user.click(wrenButton);

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(wren.id);
  });

  it("announces the match count for screen-reader users", async () => {
    const user = userEvent.setup();
    render(<BirdPicker birdId={undefined} onChange={() => {}} />);

    await user.type(screen.getByLabelText("Search species"), "wre");

    await screen.findByText("1 species found.");
  });

  it("shows a confirmation with a working clear action once a species is chosen", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<BirdPicker birdId={wren.id} onChange={onChange} />);

    expect(await screen.findByText(/Eurasian Wren/)).toBeInTheDocument();
    // The confirmation view replaces the search UI outright.
    expect(screen.queryByLabelText("Search species")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Clear Species" }));

    expect(onChange).toHaveBeenCalledWith(undefined);
  });
});
