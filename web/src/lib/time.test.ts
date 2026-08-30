import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { bumpClientUpdatedAt } from "./time";

describe("bumpClientUpdatedAt", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("advances to the current time when the previous value is well in the past", () => {
    vi.setSystemTime(new Date("2026-06-01T09:00:10.000Z"));

    const result = bumpClientUpdatedAt("2026-06-01T09:00:00.000Z");

    expect(result).toBe("2026-06-01T09:00:10.000Z");
  });

  it("bumps 1ms past the previous value when the clock has stepped backwards", () => {
    // "Now" is earlier than the value we already sent — an NTP correction,
    // for instance. The result must still be strictly newer than prev.
    vi.setSystemTime(new Date("2026-06-01T09:00:00.000Z"));

    const result = bumpClientUpdatedAt("2026-06-01T09:00:05.000Z");

    expect(result).toBe("2026-06-01T09:00:05.001Z");
  });

  it("bumps 1ms even when the previous value exactly equals now", () => {
    const now = "2026-06-01T09:00:00.000Z";
    vi.setSystemTime(new Date(now));

    const result = bumpClientUpdatedAt(now);

    expect(result).toBe("2026-06-01T09:00:00.001Z");
  });

  it("uses the current time directly when there is no previous value", () => {
    vi.setSystemTime(new Date("2026-06-01T09:00:00.000Z"));

    const result = bumpClientUpdatedAt(undefined);

    expect(result).toBe("2026-06-01T09:00:00.000Z");
  });
});
