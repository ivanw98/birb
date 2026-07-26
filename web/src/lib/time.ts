export function nowISO(): string {
  return new Date().toISOString();
}

// The contract wants "minutes ahead of UTC" (BST = +60).
// getTimezoneOffset() returns minutes BEHIND UTC (BST = -60)
export function deviceOffsetMinutes(): number {
  return new Date().getTimezoneOffset() * -1;
}

export function bumpClientUpdatedAt(prev: string | undefined): string {
  const now = Date.now();
  const floor = prev ? Date.parse(prev) + 1 : now;
  return new Date(Math.max(now, floor)).toISOString();
}

export function formatObservedAt(
  observedAt: string,
  offsetMinutes: number,
): string {
  const shiftedDate = new Date(Date.parse(observedAt) + offsetMinutes * 60_000);
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone: "UTC",
  }).format(shiftedDate);
}
