export interface AuthSession {
  authUid: string;
  email: string;
  displayName?: string;
}

export interface AuthBackend {
  getSession(): Promise<AuthSession | null>;
  getToken(): Promise<string | null>;
  signIn(): Promise<void>;
  signOut(): Promise<void>;
  onChange(cb: () => void): () => void; // Notify on sign-in/out; returns an unsubscribe function.
}
