import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RecordButton } from "./RecordButton";

describe("RecordButton", () => {
  it("exposes 'Record sighting' as its accessible name when idle, and calls onRecord when clicked", async () => {
    const user = userEvent.setup();
    const onRecord = vi.fn();
    render(<RecordButton onRecord={onRecord} busy={false} />);

    const button = screen.getByRole("button", { name: "Record sighting" });
    expect(button).toBeEnabled();

    await user.click(button);

    expect(onRecord).toHaveBeenCalledTimes(1);
  });

  it("disables itself and changes its own accessible name while busy", () => {
    render(<RecordButton onRecord={() => {}} busy={true} />);

    expect(
      screen.getByRole("button", { name: "Saving sighting..." }),
    ).toBeDisabled();
    // A screen reader exposes one name at a time so the idle name must be fully gone.
    expect(
      screen.queryByRole("button", { name: "Record sighting" }),
    ).not.toBeInTheDocument();
  });

  it("cannot be activated while busy", async () => {
    const user = userEvent.setup();
    const onRecord = vi.fn();
    render(<RecordButton onRecord={onRecord} busy={true} />);

    // A disabled native button fires no click event.
    await user.click(
      screen.getByRole("button", { name: "Saving sighting..." }),
    );

    expect(onRecord).not.toHaveBeenCalled();
  });
});
