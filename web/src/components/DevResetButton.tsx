import { Button } from "@/components/ui/button";
import { db } from "@/db/db";

async function deleteDB() {
  await db.delete();
  location.reload();
}

// Self-guarding, same as OfflineBanner. Vite statically replaces
// import.meta.env.DEV with false in a production build, so everything after
// the guard is dead code and Rollup drops it, Button import included.
export function DevResetButton() {
  if (!import.meta.env.DEV) return null;

  return (
    <Button type="button" onClick={() => void deleteDB()}>
      Reset-DB
    </Button>
  );
}
