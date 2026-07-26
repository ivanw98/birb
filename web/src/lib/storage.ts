// Ask the browser to treat our origin's storage as persistent. Without this
// (and without being installed to the home screen), iOS Safari evicts
// IndexedDB for sites unused for ~7 days — a field-notes app can't accept
// that. Call once at startup; browsers may grant silently, prompt, or refuse.
export async function ensurePersistence(): Promise<boolean> {
  if (!navigator.storage?.persist) return false;
  if (await navigator.storage.persisted()) return true;
  return navigator.storage.persist();
}

export async function storageEstimate(): Promise<{
  usageMB: number;
  quotaMB: number;
} | null> {
  if (!navigator.storage?.estimate) return null;
  const { usage, quota } = await navigator.storage.estimate();
  if (usage === undefined || quota === undefined) return null;
  return { usageMB: usage / (1024 * 1024), quotaMB: quota / (1024 * 1024) };
}
