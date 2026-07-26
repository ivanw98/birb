import type { AuthBackend, AuthSession } from "./backend";

const token = import.meta.env.VITE_DEV_TOKEN;
const authUid = import.meta.env.VITE_DEV_AUTH_UID;

const session: AuthSession | null =
  token && authUid
    ? { authUid, email: "dev@birb.local", displayName: "Dev User" }
    : null;

// Development auth: a long-lived token minted by tools/devauth, injected via
// web/.env.local.
export const devBackend: AuthBackend = {
  getSession: async () => session,
  getToken: async () => token ?? null,
  signIn: async () => {},
  signOut: async () => {},
  onChange: () => () => {},
};
