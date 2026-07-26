import { Button } from "@/components/ui/button";

export interface RecordButtonProps {
  onRecord: () => void | Promise<void>;
  busy: boolean;
}

export function RecordButton({ onRecord, busy }: RecordButtonProps) {
  return (
    <Button
      type="button"
      disabled={busy}
      onClick={() => {
        void onRecord();
      }}
      className="h-24 w-full max-w-md text-2xl"
    >
      {busy ? "Saving sighting..." : "Record sighting"}
    </Button>
  );
}
