import "@typespec/http";

using Http;

namespace Birb;

// ---------------------------------------------------------------------------
// Identifiers — Paddle/Stripe-style prefixed, k-sortable IDs (lowercase ULIDs).
// The 26-char body is a Crockford base32 ULID normalised to lowercase, so the
// values sort chronologically and carry their entity type on their face.
// Stored server-side as `text` columns with matching CHECK-constraint regexes.
// ---------------------------------------------------------------------------

@doc("Sighting identifier: `sgh_` + 26-char lowercase ULID. Generated on the client at capture time and used as the idempotency key for sync.")
@pattern("^sgh_[a-z0-9]{26}$")
@example("sgh_01j9z3x8k2m4n6p8r0s2t4v6w8")
scalar SightingId extends string;

@doc("User identifier: `usr_` + 26-char lowercase ULID. Generated server-side on first authenticated request.")
@pattern("^usr_[a-z0-9]{26}$")
@example("usr_01j9z3x8k2m4n6p8r0s2t4v6w8")
scalar UserId extends string;

@doc("Bird (species) identifier: `brd_` + 26-char lowercase ULID. Frozen values assigned once in the seed list.")
@pattern("^brd_[a-z0-9]{26}$")
@example("brd_01j9z3x8k2m4n6p8r0s2t4v6w8")
scalar BirdId extends string;

@doc("A bucket-relative Supabase Storage object path for a photo. The server stores paths only, never bytes. Bounded to 512 chars.")
@maxLength(512)
scalar PhotoPath extends string;

// ---------------------------------------------------------------------------
// Error envelope — the single error shape returned by every operation.
// ---------------------------------------------------------------------------

@doc("Machine-readable error. `code` is a stable slug for client branching; `message` is human-readable and may change.")
@error
model ApiError {
  @doc("Stable error slug, e.g. `unknown_bird_id`, `batch_too_large`, `id_conflict`, `stale_update`, `validation_failed`.")
  code: string;

  @doc("Human-readable explanation. Do not parse.")
  @maxLength(1000)
  message: string;
}

// ---------------------------------------------------------------------------
// Birds — the UK species reference list. Served for offline caching; changes
// approximately never, so responses carry a strong ETag (see routes.tsp).
// ---------------------------------------------------------------------------

@doc("A bird species in the UK reference list.")
model Bird {
  id: BirdId;

  @doc("English common name, e.g. \"Eurasian Wren\".")
  @maxLength(200)
  commonName: string;

  @doc("Scientific (binomial) name, e.g. \"Troglodytes troglodytes\".")
  @maxLength(200)
  scientificName: string;

  @doc("eBird/Clements species code, e.g. \"wilwar\". Present for interoperability; absent for a few BOU rarities whose names differ from the eBird taxonomy.")
  @maxLength(50)
  ebirdCode?: string;

  @doc("Position in the BOU British List (list order), ascending. This is list position, not the eBird global taxon order.")
  taxonomicOrder?: int32;
}

// ---------------------------------------------------------------------------
// Sightings — the core record.
//
// Write-path model (two layers, so the two write paths never fight):
//   * Capture fields (observedAt, observedAtOffsetMinutes, latitude, longitude,
//     accuracyM) are set once when the row is created and are IMMUTABLE
//     thereafter. Batch on-conflict never changes them; the enrichment PUT does
//     not accept them.
//   * Content fields (birdId, quickNote, notes, photoPaths) are mutable and
//     arbitrated by clientUpdatedAt (last-write-wins).
//
// clientUpdatedAt is ALWAYS device wall-clock (UTC), written the same way by
// both write paths — the batch sync and the enrichment PUT. The server never
// stamps it, so the LWW comparison never mixes clock domains. Clients MUST keep
// it monotonic per record (never emit a value <= the last one they sent for
// that record, even if the device clock steps backwards); the server
// additionally rejects values or observedAt more than 24h in the future.
//
// There is deliberately no server-side `status` column: pending/synced is
// purely client (Dexie) state, and any row the server holds is by definition
// synced.
// ---------------------------------------------------------------------------

@doc("A recorded bird sighting as stored and returned by the server. Optional fields are omitted from the JSON when unset (never sent as explicit null).")
model Sighting {
  id: SightingId;

  @doc("Capture field (immutable). Wall-clock instant the bird was observed, as reported by the capturing device (UTC).")
  observedAt: utcDateTime;

  @doc("Capture field (immutable). Device UTC offset in minutes at capture (e.g. 60 for British Summer Time, 0 for GMT), preserving the historical local time of the observation regardless of where the record is later viewed. Range -840..840.")
  @minValue(-840)
  @maxValue(840)
  observedAtOffsetMinutes: int32;

  @doc("Device wall-clock of the last local content edit (UTC), monotonic per record. The last-write-wins arbiter, written identically by batch sync and the enrichment PUT.")
  clientUpdatedAt: utcDateTime;

  @doc("Server receipt time (UTC). Immutable; authoritative for auditing. Not the LWW arbiter.")
  @visibility(Lifecycle.Read)
  createdAt: utcDateTime;

  @doc("Server time of the last write (UTC). Not the LWW arbiter.")
  @visibility(Lifecycle.Read)
  updatedAt: utcDateTime;

  @doc("Content field. Identified species, if chosen. Omitted when the user only left a free-text quick note.")
  birdId?: BirdId;

  @doc("Content field. Short free-text note captured in the field, e.g. \"small brown bird in reeds\". Omitted when unset.")
  @maxLength(280)
  quickNote?: string;

  @doc("Content field. Longer notes added during enrichment back at base. Omitted when unset.")
  @maxLength(5000)
  notes?: string;

  @doc("Capture field (immutable). Latitude in decimal degrees (WGS84), -90..90. Omitted if the device could not obtain a fix.")
  @minValue(-90)
  @maxValue(90)
  latitude?: float64;

  @doc("Capture field (immutable). Longitude in decimal degrees (WGS84), -180..180. Omitted if the device could not obtain a fix.")
  @minValue(-180)
  @maxValue(180)
  longitude?: float64;

  @doc("Capture field (immutable). Reported GPS accuracy radius in metres (>= 0). Omitted if unavailable.")
  @minValue(0)
  accuracyM?: float64;

  @doc("Content field. Bucket-relative Storage paths of attached photos (added via PUT). Max 10.")
  @maxItems(10)
  photoPaths: PhotoPath[];

  @doc("Present and `true` only on tombstones, which are returned only when the list was requested with `includeDeleted=true`. Omitted on live rows.")
  deleted?: boolean;
}

// ---------------------------------------------------------------------------
// Batch sync — POST /api/sightings/batch. Pushes offline-captured sightings.
// ---------------------------------------------------------------------------

@doc("""
One sighting queued offline and pushed during sync. `id` is client-generated and
is the idempotency key: re-sending the same id is safe.

The batch upsert is scoped to the caller. If the id does not exist, the row is
created and owned by the caller. If it exists and the caller owns it, the
mutable content fields (birdId, quickNote, notes) are overwritten only when the
incoming clientUpdatedAt is strictly newer than the stored one (last-write-wins);
otherwise the item is reported `stale` and nothing changes. Capture fields
(observedAt, offset, coordinates) are set at creation and never changed by a
later sync, so a re-send cannot mutate them. Photos are not handled here (see
PUT). If the id exists but is owned by another user, the item is rejected
`invalid` with code `id_conflict` — a batch never writes into another user's row.

A `deleted: true` item tombstones the row under the same last-write-wins rule;
a later item with a newer clientUpdatedAt and no `deleted` flag un-deletes it.
""")
model SightingSync {
  id: SightingId;

  @doc("Capture field, honoured only on create. Wall-clock instant the bird was observed (UTC). Rejected `invalid` if more than 24h in the future (device clock skew).")
  observedAt: utcDateTime;

  @doc("Capture field, honoured only on create. Range -840..840.")
  @minValue(-840)
  @maxValue(840)
  observedAtOffsetMinutes: int32;

  @doc("Device wall-clock of the last local content edit (UTC), monotonic per record. Drives last-write-wins. Rejected `invalid` if more than 24h in the future.")
  clientUpdatedAt: utcDateTime;

  @doc("Content field.")
  birdId?: BirdId;

  @doc("Content field.")
  @maxLength(280)
  quickNote?: string;

  @doc("Content field.")
  @maxLength(5000)
  notes?: string;

  @doc("Capture field, honoured only on create. Latitude -90..90.")
  @minValue(-90)
  @maxValue(90)
  latitude?: float64;

  @doc("Capture field, honoured only on create. Longitude -180..180.")
  @minValue(-180)
  @maxValue(180)
  longitude?: float64;

  @doc("Capture field, honoured only on create. GPS accuracy radius in metres, >= 0.")
  @minValue(0)
  accuracyM?: float64;

  @doc("Tombstone flag. `true` marks the sighting deleted; omitted or `false` marks it live, so a later edit with a newer clientUpdatedAt un-deletes. Arbitrated by clientUpdatedAt like the content fields.")
  deleted?: boolean;
}

@doc("A batch of queued sightings to synchronise. At most 100 items; larger batches are rejected whole (`batch_too_large`) — the client chunks.")
model BatchSyncRequest {
  @maxItems(100)
  sightings: SightingSync[];
}

@doc("""
Outcome for a single batch item. `created`/`updated`/`stale` are all successes
from the client's perspective (mark the local row synced): `stale` means the
server already holds an equal-or-newer version. `invalid` items (bad bird_id,
future timestamp, or an id owned by another user) must be surfaced for
correction and must not be blindly retried.
""")
model BatchItemResult {
  id: SightingId;

  @doc("`created` = inserted; `updated` = overwrote older content; `stale` = server copy was newer, no change; `invalid` = rejected (see `error`, e.g. `id_conflict`, `unknown_bird_id`, `observed_at_in_future`).")
  status: "created" | "updated" | "stale" | "invalid";

  @doc("Present only when `status` is `invalid`.")
  error?: ApiError;
}

@doc("Per-item results, one entry per submitted sighting, in request order. The overall request succeeds (HTTP 200) even when individual items are `invalid`.")
model BatchSyncResponse {
  results: BatchItemResult[];
}

// ---------------------------------------------------------------------------
// Enrichment — PUT /api/sightings/{id}. Interactive, online. Full replace of
// the mutable content fields only; capture facts are immutable and absent.
// ---------------------------------------------------------------------------

@doc("""
The complete desired state of a sighting's mutable content fields. This is a full
replace: a content field omitted from the request is cleared. Capture facts
(observedAt, offset, coordinates) are immutable and are not part of this model.
`clientUpdatedAt` is the device wall-clock of this edit (monotonic per record)
and is compared against the stored value: a stale PUT (older than stored) is
rejected 409 so the interactive UI can prompt the user to refresh, rather than
silently dropping their edit.
""")
model SightingUpdate {
  @doc("Device wall-clock of this edit (UTC), monotonic per record. The LWW arbiter; a value older than the stored one yields 409.")
  clientUpdatedAt: utcDateTime;

  @doc("Identified species, or omitted to clear.")
  birdId?: BirdId;

  @doc("Field note, or omitted to clear.")
  @maxLength(280)
  quickNote?: string;

  @doc("Detailed notes, or omitted to clear.")
  @maxLength(5000)
  notes?: string;

  @doc("The full set of photo paths after this update. Each must belong to the caller (path prefixed with their auth uid and this sighting's id). Max 10.")
  @maxItems(10)
  photoPaths: PhotoPath[];
}

// ---------------------------------------------------------------------------
// Pagination — keyset over (observedAt DESC, id DESC).
// ---------------------------------------------------------------------------

@doc("A page of sightings, newest first.")
model SightingPage {
  items: Sighting[];

  @doc("Opaque cursor for the next page; pass back as `?cursor=`. Explicitly `null` on the last page.")
  nextCursor: string | null;
}

// ---------------------------------------------------------------------------
// Account — GET /api/me. Triggers just-in-time provisioning and returns the
// caller's tier (the monetization read path).
// ---------------------------------------------------------------------------

@doc("The authenticated caller's profile. Note: user provisioning happens in the auth middleware on every request, so this never fails for a valid token — it is not the only thing that creates the user.")
model Me {
  id: UserId;

  @doc("Email from the verified token.")
  @maxLength(320)
  email: string;

  @doc("Display name from the Google profile, if present.")
  @maxLength(200)
  displayName?: string;

  @doc("Subscription tier. `free` for everyone today; the hook for future premium features and sync limits.")
  tier: "free" | "premium";
}
