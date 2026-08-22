import "@typespec/http";

using Http;

namespace Birb;

// Non-2xx shapes needing explicit modelling; ApiError (models.tsp) covers generic 4xx.

@doc("Returned by PUT /api/sightings/{id} when clientUpdatedAt is older than stored. Body is the current server state for reconciliation.")
@error
model StaleUpdate {
  @statusCode statusCode: 409;

  @doc("Always `stale_update`.")
  code: "stale_update";

  @doc("Current server state of the sighting.")
  current: Sighting;
}

@doc("304 Not Modified — If-None-Match matched the current ETag; no body.")
model BirdListNotModified {
  @statusCode statusCode: 304;
}

@doc("200 response for the bird list, carrying a strong ETag for conditional requests.")
model BirdListOk {
  @statusCode statusCode: 200;

  @doc("Strong ETag of the current list. Echo it back in If-None-Match to revalidate.")
  @header("ETag") eTag: string;

  @body birds: Bird[];
}

// ---------------------------------------------------------------------------

@doc("Create, list, and enrich bird sightings.")
@route("/sightings")
interface Sightings {
  @doc("""
  Synchronise offline-captured sightings. Idempotent caller-scoped upsert:
  new ids create, existing ids update content last-write-wins (older =
  `stale`), ids owned by another user are `invalid`/`id_conflict`. Always
  200 for a well-formed request; per-item outcomes are in the body.
  """)
  @post
  @route("/batch")
  batchSync(@body body: BatchSyncRequest): BatchSyncResponse | ApiError;

  @doc("""
  List the caller's sightings, newest first, via keyset pagination. Pass
  `nextCursor` back as `cursor`. Soft-deleted rows are excluded unless
  `includeDeleted=true`.
  """)
  @get
  list(
    @doc("Page size, 1..100. Defaults to 25.")
    @minValue(1)
    @maxValue(100)
    @query
    limit?: int32,

    @doc("Opaque cursor from a previous page's `nextCursor`.")
    @query
    cursor?: string,

    @doc("Include tombstoned sightings, marked `deleted: true`.")
    @query
    includeDeleted?: boolean,
  ): SightingPage | ApiError;

  @doc("""
  Enrich an existing sighting (full replace of content fields; capture
  facts are immutable and not accepted here). A stale write returns 409
  with current state. 404 if not found or owned by another user.
  """)
  @put
  update(@path id: SightingId, @body body: SightingUpdate): Sighting | StaleUpdate | ApiError;
}

@doc("The UK species reference list for offline caching.")
@route("/birds")
interface Birds {
  @doc("Return the full UK bird reference list. Carries a strong ETag; revalidate with If-None-Match for 304.")
  @get
  list(
    @doc("ETag from a prior response. Matching returns 304 with no body.")
    @header("If-None-Match")
    ifNoneMatch?: string,
  ): BirdListOk | BirdListNotModified | ApiError;
}

@doc("The authenticated caller's account.")
@route("/me")
interface Account {
  @doc("Return the caller's profile. Provisioning happens in auth middleware, so this is a read of already-provisioned state.")
  @get
  get(): Me | ApiError;
}

// ---------------------------------------------------------------------------

@doc("204 No Content — the membership or group is gone (or was already gone); no body.")
model GroupNoContent {
  @statusCode statusCode: 204;
}

@doc("201 Created — the new group, including the server-minted join code.")
model GroupCreated {
  @statusCode statusCode: 201;

  @body group: Group;
}

@doc("Create and manage the groups that scope who sees whose sightings.")
@route("/groups")
interface Groups {
  @doc("Create a group; the caller becomes owner and first member and the server mints the join code. NOT idempotent, so clients must disable the control while in flight. `group_limit_reached` at the owned-groups cap.")
  @post
  create(@body body: CreateGroupRequest): GroupCreated | ApiError;

  @doc("List every group the caller belongs to, owned or joined, each with its full membership. Unpaginated — bounded by the membership and member caps.")
  @get
  list(): Group[] | ApiError;

  @doc("Join by code, which the server normalises (uppercase, separators stripped) before lookup. Idempotent: re-joining returns the group unchanged. `unknown_join_code`, `group_full`, `group_limit_reached`; repeated failures are rate limited per user to 429 `join_rate_limited`.")
  @post
  @route("/join")
  join(@body body: JoinGroupRequest): Group | ApiError;

  @doc("Leave a group; idempotent, and the caller's sightings drop out of other members' feeds on their next refresh. Owners get `owner_cannot_leave` — they delete instead, since ownership never transfers.")
  @post
  @route("/{id}/leave")
  leave(@path id: GroupId): GroupNoContent | ApiError;

  @doc("Delete a group and, by cascade, its memberships; owner only (`not_group_owner`) and idempotent. Members' sightings are untouched.")
  @delete
  @route("/{id}")
  remove(@path id: GroupId): GroupNoContent | ApiError;

  @doc("Remove another member; owner only (`not_group_owner`) and idempotent. (evict rather than rotate a code)")
  @delete
  @route("/{id}/members/{userId}")
  removeMember(@path id: GroupId, @path userId: UserId): GroupNoContent | ApiError;
}

// ---------------------------------------------------------------------------

@doc("The read-only feed of sightings by everyone the caller shares a group with.")
@route("/feed")
interface Feed {
  @doc("""
  List co-members' sightings from the last 7 days, newest first, via keyset
  pagination. Pass `nextCursor` back as `cursor`. The caller's own sightings
  are never included, soft-deleted rows are excluded, and a caller in no
  groups gets an empty page.
  """)
  @get
  list(
    @doc("Page size, 1..100. Defaults to 25.")
    @minValue(1)
    @maxValue(100)
    @query
    limit?: int32,

    @doc("Opaque cursor from a previous page's `nextCursor`.")
    @query
    cursor?: string,
  ): FeedPage | ApiError;
}
