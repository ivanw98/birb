import { type PhotoStore } from "./store";
import { supabasePhotoStore } from "./supabasePhotoStore";

export const photoStore: PhotoStore = supabasePhotoStore;
export type { PhotoStore } from "./store";
