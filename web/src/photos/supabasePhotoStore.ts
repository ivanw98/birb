import { supabase } from "../lib/supabase";
import type { PhotoStore } from "./store";

export const supabasePhotoStore: PhotoStore = {
  async upload(sightingId, fileName, blob) {
    const { data: sessionData } = await supabase.auth.getSession();
    const uid = sessionData.session?.user.id;
    if (!uid) throw new Error("not signed in");
    const path = `${uid}/${sightingId}/${fileName}`;
    const { error } = await supabase.storage
      .from("sighting-photos")
      .upload(path, blob, { contentType: "image/jpeg", upsert: false });
    // 409 means the object is already there: syncPhotos() retried after a crash
    // between a successful upload and the local `uploaded` flag being written.
    if (error && error.status !== 409) throw error;
    return path;
  },
  async getRemoteUrl(path) {
    const { data, error } = await supabase.storage
      .from("sighting-photos")
      .createSignedUrl(path, 3600);
    if (error) return null;
    return data.signedUrl;
  },
};
