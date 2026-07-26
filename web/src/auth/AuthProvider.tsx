import { useEffect, useMemo, useState, type ReactNode } from "react";
import { AuthContext, type AuthState } from "./AuthContext";
import { registerTokenProvider } from "./tokenProvider";
import type { AuthBackend } from "./backend";

export function AuthProvider({
  backend,
  children,
}: {
  backend: AuthBackend;
  children: ReactNode;
}) {
  const [snapshot, setSnapshot] = useState<
    Pick<AuthState, "status" | "session">
  >({
    status: "loading",
    session: null,
  });

  useState(() => registerTokenProvider(() => backend.getToken()));

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      const session = await backend.getSession();
      if (cancelled) return;
      setSnapshot({ status: session ? "signedIn" : "signedOut", session });
    };

    void load();
    const unsubscribe = backend.onChange(() => void load());

    return () => {
      cancelled = true;
      unsubscribe();
    };
  }, [backend]);

  const value = useMemo<AuthState>(
    () => ({
      ...snapshot,
      signIn: () => backend.signIn(),
      signOut: () => backend.signOut(),
    }),
    [snapshot, backend],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
