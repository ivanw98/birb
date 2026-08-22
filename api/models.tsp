import "@typespec/http";

using Http;

namespace Birb;

// Identifiers: Paddle/Stripe-style prefixed lowercase ULIDs (k-sortable).

@doc("Sighting identifier: `sgh_` + 26-char lowercase ULID.")
@pattern("^sgh_[a-z0-9]{26}$")
@example("sgh_01j9z3x8k2m4n6p8r0s2t4v6w8")
scalar SightingId extends string;

@doc("User identifier: `usr_` + 26-char lowercase ULID.")
@pattern("^usr_[a-z0-9]{26}$")
@example("usr_01j9z3x8k2m4n6p8r0s2t4v6w8")
scalar UserId extends string;

@doc("Bird (species) identifier: `brd_` + 26-char lowercase ULID.")
@pattern("^brd_[a-z0-9]{26}$")
@example("brd_01j9z3x8k2m4n6p8r0s2t4v6w8")
scalar BirdId extends string;

@doc("Group identifier: `grp_` + 26-char lowercase ULID.")
@pattern("^grp_[a-z0-9]{26}$")
@example("grp_01j9z3x8k2m4n6p8r0s2t4v6w8")
scalar GroupId extends string;

@doc("Bucket-relative Supabase Storage object path for a photo. Max 512 chars.")
@maxLength(512)
scalar PhotoPath extends string;

// Error envelope — returned by every operation.

@doc("Machine-readable error.")
@error
model ApiError {
  @doc("Stable error slug, e.g. `unknown_bird_id`, `batch_too_large`, `id_conflict`, `stale_update`, `validation_failed`, `unknown_join_code`, `group_full`, `group_limit_reached`, `owner_cannot_leave`, `not_group_owner`, `join_rate_limited`.")
  code: string;

  @doc("Human-readable explanation. Do not parse.")
  @maxLength(1000)
  message: string;
}

// Birds — UK species reference list, served for offline caching.

@doc("A bird species in the UK reference list.")
model Bird {
  id: BirdId;

  @doc("English common name, e.g. \"Eurasian Wren\".")
  @maxLength(200)
  commonName: string;

  @doc("Scientific (binomial) name, e.g. \"Troglodytes troglodytes\".")
  @maxLength(200)
  scientificName: string;

  @doc("eBird/Clements species code, e.g. \"wilwar\". Absent for a few BOU rarities.")
  @maxLength(50)
  ebirdCode?: string;

  @doc("Position in the BOU British List (list order), ascending.")
  taxonomicOrder?: int32;
}

// Sightings — capture fields (observedAt, observedAtOffsetMinutes, latitude,
// longitude, accuracyM) are set on create and immutable thereafter. Content
// fields (birdId, quickNote, notes, photoPaths) are mutable, arbitrated by
// clientUpdatedAt (last-write-wins). No server-side `status` column —
// pending/synced is purely client state.

@doc("A recorded bird sighting. Optional fields are omitted from JSON when unset.")
model Sighting {
  id: SightingId;

  @doc("Capture field (immutable). UTC instant the bird was observed.")
  observedAt: utcDateTime;

  @doc("Capture field (immutable). Device UTC offset in minutes at capture. Range -840..840.")
  @minValue(-840)
  @maxValue(840)
  observedAtOffsetMinutes: int32;

  @doc("Device wall-clock of the last content edit (UTC), monotonic per record. LWW arbiter.")
  clientUpdatedAt: utcDateTime;

  @doc("Server receipt time (UTC). Immutable, not the LWW arbiter.")
  @visibility(Lifecycle.Read)
  createdAt: utcDateTime;

  @doc("Server time of the last write (UTC). Not the LWW arbiter.")
  @visibility(Lifecycle.Read)
  updatedAt: utcDateTime;

  @doc("Content field. Identified species, if chosen.")
  birdId?: BirdId;

  @doc("Content field. Short free-text note, e.g. \"small brown bird in reeds\".")
  @maxLength(280)
  quickNote?: string;

  @doc("Content field. Longer notes added during enrichment.")
  @maxLength(5000)
  notes?: string;

  @doc("Capture field (immutable). Latitude, WGS84, -90..90. Omitted if no GPS fix.")
  @minValue(-90)
  @maxValue(90)
  latitude?: float64;

  @doc("Capture field (immutable). Longitude, WGS84, -180..180. Omitted if no GPS fix.")
  @minValue(-180)
  @maxValue(180)
  longitude?: float64;

  @doc("Capture field (immutable). GPS accuracy radius in metres, >= 0.")
  @minValue(0)
  accuracyM?: float64;

  @doc("Content field. Storage paths of attached photos. Max 10.")
  @maxItems(10)
  photoPaths: PhotoPath[];

  @doc("`true` only on tombstones, returned only when `includeDeleted=true`.")
  deleted?: boolean;
}

// Batch sync — POST /api/sightings/batch. Pushes offline-captured sightings.

@doc("""
One sighting queued offline. `id` is client-generated and is the idempotency
key. Upsert is scoped to the caller: unknown id creates; known id owned by the
caller updates content fields last-write-wins on clientUpdatedAt (older =
`stale`, no change); id owned by another user is rejected `invalid`/`id_conflict`.
Capture fields are honoured only on create. `deleted: true` tombstones under
the same LWW rule.
""")
model SightingSync {
  id: SightingId;

  @doc("Capture field, honoured only on create. Rejected `invalid` if >24h in the future.")
  observedAt: utcDateTime;

  @doc("Capture field, honoured only on create. Range -840..840.")
  @minValue(-840)
  @maxValue(840)
  observedAtOffsetMinutes: int32;

  @doc("LWW arbiter. Rejected `invalid` if >24h in the future.")
  clientUpdatedAt: utcDateTime;

  @doc("Content field.")
  birdId?: BirdId;

  @doc("Content field.")
  @maxLength(280)
  quickNote?: string;

  @doc("Content field.")
  @maxLength(5000)
  notes?: string;

  @doc("Capture field, honoured only on create. -90..90.")
  @minValue(-90)
  @maxValue(90)
  latitude?: float64;

  @doc("Capture field, honoured only on create. -180..180.")
  @minValue(-180)
  @maxValue(180)
  longitude?: float64;

  @doc("Capture field, honoured only on create. >= 0.")
  @minValue(0)
  accuracyM?: float64;

  @doc("Tombstone flag. Arbitrated by clientUpdatedAt like content fields.")
  deleted?: boolean;
}

@doc("A batch of queued sightings. Max 100 items; larger batches rejected whole (`batch_too_large`).")
model BatchSyncRequest {
  @maxItems(100)
  sightings: SightingSync[];
}

@doc("Outcome for one batch item. `created`/`updated`/`stale` are successes; `invalid` must be surfaced and not retried.")
model BatchItemResult {
  id: SightingId;

  @doc("`created` = inserted; `updated` = overwrote older content; `stale` = server copy newer, no change; `invalid` = rejected (see `error`).")
  status: "created" | "updated" | "stale" | "invalid";

  @doc("Present only when `status` is `invalid`.")
  error?: ApiError;
}

@doc("Per-item results, one per submitted sighting, in request order. Overall request succeeds (200) even with invalid items.")
model BatchSyncResponse {
  results: BatchItemResult[];
}

// Enrichment — PUT /api/sightings/{id}. Full replace of content fields only.

@doc("""
Full replace of a sighting's mutable content fields; an omitted field is
cleared. Capture facts are immutable and not part of this model.
`clientUpdatedAt` older than the stored value yields 409.
""")
model SightingUpdate {
  @doc("LWW arbiter. Older than stored yields 409.")
  clientUpdatedAt: utcDateTime;

  @doc("Identified species, or omitted to clear.")
  birdId?: BirdId;

  @doc("Field note, or omitted to clear.")
  @maxLength(280)
  quickNote?: string;

  @doc("Detailed notes, or omitted to clear.")
  @maxLength(5000)
  notes?: string;

  @doc("Full set of photo paths after this update. Each must belong to the caller. Max 10.")
  @maxItems(10)
  photoPaths: PhotoPath[];
}

// Pagination — keyset over (observedAt DESC, id DESC).

@doc("A page of sightings, newest first.")
model SightingPage {
  items: Sighting[];

  @doc("Opaque cursor for the next page. `null` on the last page.")
  nextCursor: string | null;
}

// Groups — social layer. Membership is the only sharing mechanism; joining
// shares the feed projection with other members (no per-sighting opt-out in
// v1).

@doc("A member of a group, as exposed to other members.")
model GroupMember {
  id: UserId;

  @doc("Display name from the identity provider. Omitted if unset.")
  @maxLength(200)
  name?: string;

  @doc("True for the member who created the group. Ownership never transfers.")
  isOwner: boolean;
}

@doc("A named group and its full membership, as returned to a member.")
model Group {
  id: GroupId;

  @doc("Human-chosen group name, e.g. \"Sunday walkers\". Trimmed server-side, 1..100 chars.")
  @minLength(1)
  @maxLength(100)
  name: string;

  @doc("Canonical join code: 8 uppercase chars, no separators, e.g. \"BQTX7RKM\". Excludes 0, O, 1, I, L, U.")
  @minLength(8)
  @maxLength(8)
  joinCode: string;

  @doc("True when the caller owns this group.")
  isOwner: boolean;

  @doc("Every member including the caller. Owner first, then join time ascending.")
  members: GroupMember[];
}

@doc("Request to create a group. The caller becomes its owner and first member.")
model CreateGroupRequest {
  @doc("Group name. Trimmed server-side before the length check.")
  @minLength(1)
  @maxLength(100)
  name: string;
}

@doc("Request to join a group by code.")
model JoinGroupRequest {
  @doc("Join code in any casing/separators — server normalises before lookup.")
  @maxLength(32)
  code: string;
}

// Feed — the read-only projection of a co-member's sighting.

@doc("One co-member's sighting, as exposed to everyone sharing a group with them.")
model FeedItem {
  @doc("The projected sighting. Never one of the caller's own.")
  sightingId: SightingId;

  @doc("Identified species, if the observer chose one.")
  birdId?: BirdId;

  @doc("Observer's display name. Omitted if unset.")
  @maxLength(200)
  authorName?: string;

  @doc("UTC instant the bird was observed.")
  observedAt: utcDateTime;

  @doc("Nearest settlement. Omitted when the sighting has no coordinates or nothing lies within 30 km.")
  @maxLength(200)
  placeName?: string;
}

@doc("A page of co-members' sightings from the last 7 days, newest first.")
model FeedPage {
  items: FeedItem[];

  @doc("Opaque cursor for the next page. `null` on the last page.")
  nextCursor: string | null;
}

// Account — GET /api/me. Also triggers just-in-time user provisioning.

@doc("The authenticated caller's profile.")
model Me {
  id: UserId;

  @doc("Email from the verified token.")
  @maxLength(320)
  email: string;

  @doc("Display name from the Google profile, if present.")
  @maxLength(200)
  displayName?: string;

  @doc("Subscription tier. `free` for everyone today.")
  tier: "free" | "premium";
}
