import type { ReactNode } from "react";

type Tone = "info" | "success" | "danger";

interface StatusBannerProps {
  tone: Tone;
  children: ReactNode;
  // Renders a standard Dismiss button. Dismissal is always an explicit user
  // action: the charter forbids timers, not closing.
  onDismiss?: () => void;
}

const TONE_CLASSES: Record<Tone, string> = {
  info: "border-primary/30 bg-primary/5",
  success: "border-success/30 bg-success/5",
  danger: "border-danger/30 bg-danger/5",
};

const TONE_LABEL: Record<Tone, string> = {
  info: "Info",
  success: "Success",
  danger: "Problem",
};

export function StatusBanner({ tone, children, onDismiss }: StatusBannerProps) {
  return (
    <div
      role={tone === "danger" ? "alert" : "status"}
      aria-live={tone === "danger" ? undefined : "polite"}
      className={`w-full max-w-md rounded-lg border-2 px-4 py-3 text-lg text-ink ${TONE_CLASSES[tone]}`}
    >
      <strong>{TONE_LABEL[tone]}: </strong>
      {children}
      {onDismiss && (
        <button
          type="button"
          onClick={onDismiss}
          className="ml-3 h-12 rounded-md border border-slate-300 bg-white px-4 text-ink"
        >
          Dismiss
        </button>
      )}
    </div>
  );
}
