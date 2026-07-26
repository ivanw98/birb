import { supabase } from "../lib/supabase";
import type { AuthBackend, AuthSession } from "./backend";

function toSession(
  s: {
    access_token: string;
    user: {
      id: string;
      email?: string;
      user_metadata: Record<string, unknown>;
    };
  } | null,
): AuthSession | null {
  if (!s) return null;
  return {
    authUid: s.user.id,
    email: s.user.email ?? "",
    displayName:
      typeof s.user.user_metadata.name === "string"
        ? s.user.user_metadata.name
        : undefined,
  };
}

export const supabaseBackend: AuthBackend = {
  async getSession() {
    const { data } = await supabase.auth.getSession();
    return toSession(data.session);
  },
  // getSession() transparently refreshes a near-expiry token
  async getToken() {
    const { data } = await supabase.auth.getSession();
    return data.session?.access_token ?? null;
  },
  async signIn() {
    await supabase.auth.signInWithOAuth({
      provider: "google",
      options: { redirectTo: window.location.origin },
    });
  },
  async signOut() {
    await supabase.auth.signOut();
  },
  onChange(cb) {
    const { data } = supabase.auth.onAuthStateChange(() => cb());
    return () => data.subscription.unsubscribe();
  },
};
