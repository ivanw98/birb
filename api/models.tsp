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

@doc("Bird (species) identifier: `brd_` + 26-char lowercase ULID. Frozen values assigned by the seed script.")
@pattern("^brd_[a-z0-9]{26}$")
@example("brd_01j9z3x8k2m4n6p8r0s2t4v6w8")
scalar BirdId extends string;

// ---------------------------------------------------------------------------
// Error envelope — the single error shape returned by every operation.
// ---------------------------------------------------------------------------

@doc("Machine-readable error. `code` is a stable slug for client branching; `message` is human-readable and may change.")
@error
model ApiError {
  @doc("Stable error slug, e.g. `unknown_bird_id`, `batch_too_large`, `id_conflict`, `validation_failed`.")
  code: string;

  @doc("Human-readable explanation. Do not parse.")
  message: string;
}

// ---------------------------------------------------------------------------
// Birds — the UK species reference list. Served for offline caching; changes
// approximately never, so responses carry a strong ETag.
// ---------------------------------------------------------------------------

@doc("A bird species in the UK reference list.")
model Bird {
  id: BirdId;

  @doc("English common name, e.g. \"Eurasian Wren\".")
  commonName: string;

  @doc("Scientific (binomial) name, e.g. \"Troglodytes troglodytes\".")
  scientificName: string;

  @doc("eBird/Clements species code, e.g. \"wilwar\". Present for interoperability; may be absent for a few rarities.")
  ebirdCode?: string;

  @doc("Taxonomic sort order (eBird taxon order). Ascending gives a conventional field-guide ordering.")
  taxonomicOrder?: int32;
}

// ---------------------------------------------------------------------------
// Sightings — the core record. See the batch/list/update operations for the
// full lifecycle. Note there is deliberately no server-side `status` field:
// pending/synced is purely client (Dexie) state, and any row the server holds
// is by definition synced.
// ---------------------------------------------------------------------------

@doc("A recorded bird sighting as stored and returned by the server.")
model Sighting {
  id: SightingId;

  @doc("Wall-clock instant the bird was observed, as reported by the capturing device (UTC). Immutable after creation.")
  observedAt: utcDateTime;

  @doc("Device UTC offset in minutes at capture time (e.g. 60 for British Summer Time, 0 for GMT). Preserves the historical local time of the observation regardless of where the record is later viewed. Range -840..840.")
  @minValue(-840)
  @maxValue(840)
  observedAtOffsetMinutes: int32;

  @doc("Device wall-clock of the last local edit (UTC). Drives last-write-wins on batch sync. Set to server time by the enrichment PUT.")
  clientUpdatedAt: utcDateTime;

  @doc("Server receipt time (UTC). Immutable; authoritative for auditing.")
  @visibility(Lifecycle.Read)
  createdAt: utcDateTime;

  @doc("Server time of the last write (UTC).")
  @visibility(Lifecycle.Read)
  updatedAt: utcDateTime;

  @doc("Identified species, if chosen. Null when the user only left a free-text quick note.")
  birdId?: BirdId;

  @doc("Short free-text note captured in the field, e.g. \"small brown bird in reeds\".")
  quickNote?: string;

  @doc("Longer notes added during enrichment back at base.")
  notes?: string;

  @doc("Latitude in decimal degrees (WGS84), -90..90. Null if the device could not obtain a fix.")
  @minValue(-90)
  @maxValue(90)
  latitude?: float64;

  @doc("Longitude in decimal degrees (WGS84), -180..180. Null if the device could not obtain a fix.")
  @minValue(-180)
  @maxValue(180)
  longitude?: float64;

  @doc("Reported GPS accuracy radius in metres, if available.")
  accuracyM?: float64;

  @doc("Bucket-relative Supabase Storage paths of attached photos, e.g. `<auth_uid>/<sighting_id>/<file>`. The API stores paths only; the client resolves signed URLs. Max 10.")
  @maxItems(10)
  photoPaths: string[];
}

// ---------------------------------------------------------------------------
// Batch sync — POST /api/sightings/batch.
// ---------------------------------------------------------------------------

@doc("""
One sighting queued offline and pushed during sync. `id` is client-generated
and is the idempotency key: re-sending the same id is safe. The server performs
a guarded upsert — an existing row is overwritten only when the incoming
`clientUpdatedAt` is strictly newer than the stored one (last-write-wins),
so an offline correction is never lost to a stale retry.
""")
model SightingSync {
  id: SightingId;

  @doc("Wall-clock instant the bird was observed (UTC). Rejected as invalid if more than 24h in the future (device clock skew).")
  observedAt: utcDateTime;

  @minValue(-840)
  @maxValue(840)
  observedAtOffsetMinutes: int32;

  @doc("Device wall-clock of the last local edit (UTC). Also rejected if more than 24h in the future.")
  clientUpdatedAt: utcDateTime;

  birdId?: BirdId;
  quickNote?: string;
  notes?: string;

  @minValue(-90)
  @maxValue(90)
  latitude?: float64;

  @minValue(-180)
  @maxValue(180)
  longitude?: float64;

  accuracyM?: float64;

  @doc("Photo paths, if any were already uploaded. Usually empty at capture time; photos are typically attached later via PUT.")
  @maxItems(10)
  photoPaths?: string[];
}

@doc("A batch of queued sightings to synchronise. At most 100 items; larger batches are rejected whole (`batch_too_large`) — the client chunks.")
model BatchSyncRequest {
  @maxItems(100)
  sightings: SightingSync[];
}

@doc("""
Outcome for a single batch item. `created`/`updated`/`stale` are all successes
from the client's perspective (mark the local row synced): `stale` means the
server already holds an equal-or-newer version. `invalid` items must be
surfaced for correction and must not be blindly retried.
""")
model BatchItemResult {
  id: SightingId;

  @doc("`created` = inserted; `updated` = overwrote an older version; `stale` = server copy was newer, no change; `invalid` = rejected (see `error`).")
  status: "created" | "updated" | "stale" | "invalid";

  @doc("Present only when `status` is `invalid`.")
  error?: ApiError;
}

@doc("Per-item results, one entry per submitted sighting, in request order. The overall request succeeds (HTTP 200) even when individual items are `invalid`.")
model BatchSyncResponse {
  results: BatchItemResult[];
}

// ---------------------------------------------------------------------------
// Enrichment — PUT /api/sightings/{id}. Full replace of the enrichable field
// set only. Capture facts (observedAt, offset, coordinates) are immutable and
// are deliberately absent from this model.
// ---------------------------------------------------------------------------

@doc("The complete desired state of a sighting's enrichable fields. Sent in full on every update; omitted-and-null are equivalent (the field is cleared).")
model SightingUpdate {
  birdId?: BirdId;
  quickNote?: string;
  notes?: string;

  @doc("The full set of photo paths after this update. Each must belong to the caller (path prefixed with their auth uid and this sighting's id). Max 10.")
  @maxItems(10)
  photoPaths: string[];
}

// ---------------------------------------------------------------------------
// Pagination — keyset over (observedAt DESC, id DESC).
// ---------------------------------------------------------------------------

@doc("A page of sightings, newest first. `nextCursor` is null on the last page.")
model SightingPage {
  items: Sighting[];

  @doc("Opaque cursor for the next page; pass back as `?cursor=`. Null means the end of the list.")
  nextCursor?: string;
}

// ---------------------------------------------------------------------------
// Account — GET /api/me. Triggers just-in-time provisioning and returns the
// caller's tier (the monetization read path).
// ---------------------------------------------------------------------------

@doc("The authenticated caller's profile.")
model Me {
  id: UserId;
  email: string;
  displayName?: string;

  @doc("Subscription tier. `free` for everyone today; the hook for future premium features and sync limits.")
  tier: "free" | "premium";
}
