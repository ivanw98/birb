import { monotonicFactory } from "ulidx";

const ulid = monotonicFactory();

export function newSightingID(): string {
  return `sgh_${ulid().toLowerCase()}`;
}

export function newPhotoFileName(): string {
  return `${ulid().toLowerCase()}.jpg`;
}

export function newRecordingFileName(extension: string): string {
  return `${ulid().toLowerCase()}.${extension}`;
}
