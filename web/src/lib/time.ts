export function nowISO(): string {
  return new Date().toISOString();
}

// The contract wants "minutes ahead of UTC" (BST = +60).
// getTimezoneOffset() returns minutes BEHIND UTC (BST = -60)
export function deviceOffsetMinutes(): number {
  return new Date().getTimezoneOffset() * -1;
}

function sameCalendarDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

export function formatFeedTime(observedAt: string): string {
  const then = new Date(observedAt);
  const now = new Date();

  const time = new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).format(then);

  if (sameCalendarDay(then, now)) return `Today ${time}`;

  const yesterday = new Date(
    now.getFullYear(),
    now.getMonth(),
    now.getDate() - 1,
  );

  if (sameCalendarDay(then, yesterday)) return `Yesterday ${time}`;

  const date = new Intl.DateTimeFormat(undefined, {
    weekday: "short",
    month: "short",
    day: "numeric",
  }).format(then);

  return `${date} ${time}`;
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
