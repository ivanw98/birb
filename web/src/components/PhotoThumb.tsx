import { useEffect, useState } from "react";
import { photoStore } from "@/photos";
import { localBlobForPath } from "@/photos/store";

// Exactly one source, never both: a server `path` to resolve, or a `blob` we
// already hold (a queued photo that has no path yet).
export type PhotoThumbProps = { birdName?: string } & (
  | { path: string; blob?: undefined }
  | { blob: Blob; path?: undefined }
);

type ThumbState =
  { kind: "loading" } | { kind: "ready"; url: string } | { kind: "missing" };

// Sources tried in order:
// a blob handed straight to us (a queued photo, not yet uploaded),
// a blob held locally for this path (everything else on device),
// a remote signed URL (a photo taken on ANOTHER device),
// or a placeholder.
export function PhotoThumb({ path, blob, birdName }: PhotoThumbProps) {
  const [thumb, setThumb] = useState<ThumbState>({ kind: "loading" });

  useEffect(() => {
    let objectUrl: string | null = null;
    let cancelled = false;
    setThumb({ kind: "loading" });

    (async () => {
      const local = blob ?? (path ? await localBlobForPath(path) : undefined);
      if (cancelled) return;
      if (local) {
        objectUrl = URL.createObjectURL(local);
        setThumb({ kind: "ready", url: objectUrl });
        return;
      }
      const remote = path ? await photoStore.getRemoteUrl(path) : null;
      if (cancelled) return;
      setThumb(remote ? { kind: "ready", url: remote } : { kind: "missing" });
    })();

    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [path, blob]);

  if (thumb.kind === "missing") {
    return (
      <div className="flex h-24 w-24 items-center justify-center rounded-md border border-slate-300 bg-slate-100 p-1 text-center text-xs text-muted">
        Photo not on this device
      </div>
    );
  }

  if (thumb.kind === "loading") {
    return (
      <div
        aria-hidden="true"
        className="h-24 w-24 animate-pulse rounded-md border border-slate-300 bg-slate-100"
      />
    );
  }

  return (
    <img
      src={thumb.url}
      alt={birdName ?? "sighting photo"}
      className="h-24 w-24 rounded-md border border-slate-300 object-cover"
    />
  );
}
