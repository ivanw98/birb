import { db, type LocalPhoto } from "@/db/db";
import { newPhotoFileName } from "@/lib/id";
import { compressImage } from "@/lib/photos";
import { syncNow } from "@/sync/syncEngine";
import { useId, useState, type ChangeEvent } from "react";

export const MAX_PHOTOS = 10;

export interface PhotoCaptureProps {
  sightingId: string;
  remaining: number;
  // Held shut while the parent has a write of its own in flight: attaching
  // fires syncNow(), whose PUT would race the parent's.
  disabled?: boolean;
}

export function PhotoCapture({
  sightingId,
  remaining,
  disabled = false,
}: PhotoCaptureProps) {
  const inputId = useId();
  const hintId = `${inputId}-hint`;

  const [busy, setBusy] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);
  const atLimit = remaining <= 0;

  const handleFiles = async (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    // picking the same file again still fires onChange
    e.target.value = "";
    if (!file) return;

    setBusy(true);
    setFeedback(null);
    try {
      // Compress BEFORE sending to IndexedDB
      const blob = await compressImage(file);

      const photoItem: LocalPhoto = {
        sightingId,
        fileName: newPhotoFileName(),
        blob,
        uploaded: 0,
      };
      await db.photos.add(photoItem);

      setFeedback(`${formatBytes(file.size)} \u2192 ${formatBytes(blob.size)}`);
      // fire-and-forget
      void syncNow();
    } finally {
      setBusy(false);
    }
  };

  return (
    <div>
      <label htmlFor={inputId} className="mb-2 block font-medium text-ink">
        Add a photo{atLimit ? "" : ` (${remaining} of ${MAX_PHOTOS} left)`}
      </label>
      <input
        id={inputId}
        type="file"
        accept="image/*"
        disabled={busy || disabled || atLimit}
        aria-describedby={hintId}
        onChange={(e) => void handleFiles(e)}
        className="block w-full text-base text-muted file:mr-3 file:h-12 file:cursor-pointer file:rounded-md file:border file:border-primary file:bg-white file:px-4 file:text-base file:font-medium file:text-primary disabled:cursor-not-allowed disabled:opacity-50"
      ></input>
      <p
        id={hintId}
        aria-live="polite"
        className="mt-1 min-h-6 text-sm text-muted"
      >
        {atLimit
          ? `This sighting already has ${MAX_PHOTOS} photos, the most the app keeps per sighting.`
          : busy
            ? "Compressing..."
            : disabled
              ? "Finishing saving first. One moment."
              : feedback
                ? `Compressed ${feedback}`
                : ""}
      </p>
    </div>
  );
}

function formatBytes(bytes: number): string {
  return bytes >= 1024 * 1024
    ? `${(bytes / (1024 * 1024)).toFixed(1)}MB`
    : `${Math.round(bytes / 1024)}KB`;
}
