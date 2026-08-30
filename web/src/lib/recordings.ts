/*
 * CANDIDATES represents the audio formats that need probing for at runtime.
 * Different browsers hold audio in different formats, and MediaRecorder has no way to request one.
 * Pick the first one that matches.
 */
const CANDIDATES: { mimeType: string; extension: string }[] = [
  { mimeType: "audio/webm;codecs=opus", extension: "webm" },
  { mimeType: "audio/ogg;codecs=opus", extension: "ogg" },
  { mimeType: "audio/mp4", extension: "m4a" },
];

export const MAX_RECORDING_SECONDS = 60;

export interface RecordingFormat {
  mimeType: string;
  extension: string;
}

// returns null on a browser with no usable format at all.
export function pickRecordingMimeType(): RecordingFormat | null {
  if (typeof MediaRecorder === "undefined") return null;
  return (
    CANDIDATES.find((c) => MediaRecorder.isTypeSupported(c.mimeType)) ?? null
  );
}
