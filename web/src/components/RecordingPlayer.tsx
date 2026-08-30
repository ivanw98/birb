import { useEffect, useState } from "react";
import { recordingStore } from "@/recordings";
import { localRecordingBlobForPath } from "@/recordings/store";

export type RecordingPlayerProps = { label: string } & (
  { path: string; blob?: undefined } | { blob: Blob; path?: undefined }
);

type PlayerState =
  { kind: "loading" } | { kind: "ready"; url: string } | { kind: "missing" };

export function RecordingPlayer({ path, blob, label }: RecordingPlayerProps) {
  const [player, setPlayer] = useState<PlayerState>({ kind: "loading" });

  useEffect(() => {
    let objectUrl: string | null = null;
    let cancelled = false;
    setPlayer({ kind: "loading" });

    (async () => {
      const local =
        blob ?? (path ? await localRecordingBlobForPath(path) : undefined);
      if (cancelled) return;
      if (local) {
        objectUrl = URL.createObjectURL(local);
        setPlayer({ kind: "ready", url: objectUrl });
        return;
      }
      const remote = path ? await recordingStore.getRemoteUrl(path) : null;
      if (cancelled) return;
      setPlayer(remote ? { kind: "ready", url: remote } : { kind: "missing" });
    })();

    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [path, blob]);

  if (player.kind === "missing") {
    return <p className="text-sm text-muted">{label}: not on this device</p>;
  }

  if (player.kind === "loading") {
    return (
      <div
        aria-hidden="true"
        className="h-12 w-full max-w-xs animate-pulse rounded-md border border-slate-300 bg-slate-100"
      />
    );
  }

  return (
    <audio
      controls
      src={player.url}
      aria-label={label}
      className="h-12 w-full max-w-xs"
    />
  );
}
