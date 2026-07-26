import "@typespec/http";

using Http;

namespace Birb;

// ---------------------------------------------------------------------------
// Non-2xx response shapes that need explicit modelling (so codegen produces
// the right surface). ApiError (from models.tsp) covers the generic 4xx.
// ---------------------------------------------------------------------------

@doc("Returned by PUT /api/sightings/{id} when the submitted clientUpdatedAt is older than the stored one. The body is the current server state so an interactive UI can reconcile and prompt the user to refresh rather than silently losing their edit.")
@error
model StaleUpdate {
  @statusCode statusCode: 409;

  @doc("Stable slug: always `stale_update` for this response.")
  code: "stale_update";

  @doc("The current server state of the sighting the caller tried to update.")
  current: Sighting;
}

@doc("304 Not Modified — the client's If-None-Match matched the current ETag; no body.")
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
  Synchronise a batch of offline-captured sightings. Idempotent, caller-scoped
  upsert: new ids are created; existing ids owned by the caller have their
  mutable content fields updated last-write-wins on clientUpdatedAt (older =
  reported `stale`, no change); capture fields are never mutated after creation;
  an id owned by another user is rejected `invalid`/`id_conflict`. Always returns
  200 for a well-formed, authenticated request — per-item outcomes are in the
  body. The whole request is rejected only for malformed JSON or more than 100
  items (`batch_too_large`).
  """)
  @post
  @route("/batch")
  batchSync(@body body: BatchSyncRequest): BatchSyncResponse | ApiError;

  @doc("""
  List the caller's sightings, newest first, via keyset pagination. Pass the
  previous response's `nextCursor` as `cursor` to page. Soft-deleted rows are
  excluded (deletion has no client-facing sync mechanism in v0.1).
  """)
  @get
  list(
    @doc("Page size, 1..100. Defaults to 25 when omitted.")
    @minValue(1)
    @maxValue(100)
    @query
    limit?: int32,

    @doc("Opaque cursor from a previous page's `nextCursor`. Omit for the first page.")
    @query
    cursor?: string,
  ): SightingPage | ApiError;

  @doc("""
  Enrich an existing sighting (full replace of the mutable content fields;
  capture facts are immutable and not accepted here). Interactive and online:
  a stale write (submitted clientUpdatedAt older than stored) returns 409 with
  the current state so the UI can reconcile. Returns 404 if the sighting does
  not exist or belongs to another user. Photo paths must be owned by the caller.
  """)
  @put
  update(@path id: SightingId, @body body: SightingUpdate): Sighting | StaleUpdate | ApiError;
}

@doc("The UK species reference list for offline caching.")
@route("/birds")
interface Birds {
  @doc("""
  Return the full UK bird reference list. The response carries a strong ETag; the
  list changes approximately never, so clients should cache it and revalidate
  with If-None-Match, receiving 304 when unchanged.
  """)
  @get
  list(
    @doc("The ETag from a prior response. If it matches the current list, the server returns 304 with no body.")
    @header("If-None-Match")
    ifNoneMatch?: string,
  ): BirdListOk | BirdListNotModified | ApiError;
}

@doc("The authenticated caller's account.")
@route("/me")
interface Account {
  @doc("Return the caller's profile. User provisioning occurs in the auth middleware on every authenticated request, so this is a read of already-provisioned state; it is also the client's source of truth for the current tier.")
  @get
  get(): Me | ApiError;
}
