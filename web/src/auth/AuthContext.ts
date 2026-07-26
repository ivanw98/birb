import { createContext, useContext } from "react";
import type { AuthSession } from "./backend";

export interface AuthState {
  status: "loading" | "signedIn" | "signedOut";
  session: AuthSession | null;
  signIn: () => Promise<void>;
  signOut: () => Promise<void>;
}

export const AuthContext = createContext<AuthState | null>(null);

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used inside <AuthProvider>");
  return ctx;
}
