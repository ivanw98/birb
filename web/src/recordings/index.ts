import type { RecordingStore } from "./store";
import { supabaseRecordingStore } from "./supabaseRecordingStore";

export const recordingStore: RecordingStore = supabaseRecordingStore;
export type { RecordingStore } from "./store";
