import { useSyncExternalStore } from "react";

function subscribe(callbackFn: () => void): () => void {
  window.addEventListener("online", callbackFn);
  window.addEventListener("offline", callbackFn);

  return () => {
    window.removeEventListener("online", callbackFn);
    window.removeEventListener("offline", callbackFn);
  };
}

export function useOnline(): boolean {
  const getSnapshot = () => navigator.onLine;
  return useSyncExternalStore(subscribe, getSnapshot);
}
