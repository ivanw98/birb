import {
  getSyncState,
  subscribeSync,
  type SyncEngineState,
} from "@/sync/syncEngine";
import { useSyncExternalStore } from "react";

export function useSyncState(): SyncEngineState {
  return useSyncExternalStore(subscribeSync, getSyncState);
}
