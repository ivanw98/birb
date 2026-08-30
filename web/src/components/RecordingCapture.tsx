import { db, type LocalRecording } from "@/db/db";
import { newRecordingFileName } from "@/lib/id";
import { MAX_RECORDING_SECONDS, pickRecordingMimeType } from "@/lib/recordings";
import { syncNow } from "@/sync/syncEngine";
import { useEffect, useId, useRef, useState } from "react";

export const MAX_RECORDINGS = 5;

export interface RecordingCaptureProps {
  sightingId: string;
  remaining: number;
  disabled?: boolean;
}

type RecordState =
  | { kind: "idle" }
  | { kind: "recording"; seconds: number }
  | { kind: "denied" }
  | { kind: "unsupported" };

export function RecordingCapture({
  sightingId,
  remaining,
  disabled = false,
}: RecordingCaptureProps) {
  const controlId = useId();
  const hintId = `${controlId}-hint`;

  const [state, setState] = useState<RecordState>({ kind: "idle" });
  const recorderRef = useRef<MediaRecorder | null>(null);
  const chunksRef = useRef<Blob[]>([]);
  const streamRef = useRef<MediaStream | null>(null);
  const timerRef = useRef<number | null>(null);
  const formatRef = useRef<{ mimeType: string; extension: string } | null>(
    null,
  );

  const atLimit = remaining <= 0;
  const recording = state.kind === "recording";

  // Closing the dialog mid-recording must not leave the microphone lit.
  useEffect(() => {
    return () => {
      clearInterval(timerRef.current ?? undefined);
      streamRef.current?.getTracks().forEach((track) => track.stop());
    };
  }, []);

  const startRecording = async () => {
    const format = pickRecordingMimeType();
    if (!format) {
      setState({ kind: "unsupported" });
      return;
    }

    let stream: MediaStream;
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    } catch {
      // Denied, no microphone, or an insecure context — getUserMedia collapses
      // all three into one rejection, so one message covers all three.
      setState({ kind: "denied" });
      return;
    }

    streamRef.current = stream;
    formatRef.current = format;
    chunksRef.current = [];

    const recorder = new MediaRecorder(stream, { mimeType: format.mimeType });
    recorder.ondataavailable = (event) => {
      if (event.data.size > 0) chunksRef.current.push(event.data);
    };
    recorder.onstop = () => {
      void finishRecording();
    };
    recorderRef.current = recorder;
    recorder.start();
    setState({ kind: "recording", seconds: 0 });

    let elapsed = 0;
    timerRef.current = setInterval(() => {
      // Without this guard a tick landing after stop() but before the async
      // onstop pushes elapsed past the cap: the countdown reads "-1s left".
      if (recorderRef.current?.state !== "recording") return;
      elapsed += 1;
      setState({ kind: "recording", seconds: elapsed });
      if (elapsed >= MAX_RECORDING_SECONDS) recorderRef.current.stop();
    }, 1000);
  };

  const stopRecording = () => {
    recorderRef.current?.stop();
  };

  const finishRecording = async () => {
    clearInterval(timerRef.current ?? undefined);
    timerRef.current = null;
    streamRef.current?.getTracks().forEach((track) => track.stop());
    streamRef.current = null;

    const format = formatRef.current;
    const chunks = chunksRef.current;
    chunksRef.current = [];
    if (!format || chunks.length === 0) {
      setState({ kind: "idle" });
      return;
    }

    const blob = new Blob(chunks, { type: format.mimeType });
    const item: LocalRecording = {
      sightingId,
      fileName: newRecordingFileName(format.extension),
      blob,
      mimeType: format.mimeType,
      uploaded: 0,
    };
    await db.recordings.add(item);
    setState({ kind: "idle" });
    void syncNow();
  };

  return (
    <div>
      <p className="mb-2 font-medium text-ink">
        Add a recording
        {atLimit ? "" : ` (${remaining} of ${MAX_RECORDINGS} left)`}
      </p>
      <button
        type="button"
        id={controlId}
        onClick={() => (recording ? stopRecording() : void startRecording())}
        disabled={disabled || (atLimit && !recording)}
        aria-describedby={hintId}
        className="h-12 w-full rounded-md border border-primary bg-white text-base font-medium text-primary disabled:cursor-not-allowed disabled:opacity-50"
      >
        {recording
          ? `Stop recording (${MAX_RECORDING_SECONDS - state.seconds}s left)`
          : "Start recording"}
      </button>
      <p
        id={hintId}
        aria-live="polite"
        className="mt-1 min-h-6 text-sm text-muted"
      >
        {atLimit && !recording
          ? `This sighting already has ${MAX_RECORDINGS} recordings, the most the app keeps per sighting.`
          : state.kind === "denied"
            ? "Microphone access was denied. Check your browser's site settings and try again."
            : state.kind === "unsupported"
              ? "This browser can't record audio."
              : disabled
                ? "Finishing saving first. One moment."
                : recording
                  ? "Recording…"
                  : ""}
      </p>
    </div>
  );
}
