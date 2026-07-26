import { Button } from "@/components/ui/button";
import { useAuth } from "../auth/AuthContext";

export function SignInScreen() {
  const { signIn } = useAuth();

  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-8 bg-white p-6 text-center">
      <div className="max-w-sm space-y-3">
        <h1 className="text-2xl font-semibold text-ink">birb</h1>
        <p className="text-lg text-muted">
          Sign in with your Google account so your sightings sync to the server
          and your photos have somewhere durable to live. Until you do,
          everything you capture stays safely on this device.
        </p>
      </div>
      <Button
        type="button"
        onClick={() => void signIn()}
        className="h-14 w-full max-w-sm text-lg font-medium"
      >
        Sign in with Google
      </Button>
    </main>
  );
}
