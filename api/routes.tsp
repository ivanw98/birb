import "@typespec/http";

using Http;

namespace Birb;

@doc("Create, list, and enrich bird sightings.")
@route("/sightings")
interface Sightings {
  @doc("""
  Synchronise a batch of offline-captured sightings (idempotent upsert,
  last-write-wins on `clientUpdatedAt`). Always returns 200 for a well-formed,
  authenticated request; per-item outcomes are in the response body. The whole
  request is rejected only for malformed JSON or more than 100 items
  (`batch_too_large`).
  """)
  @post
  @route("/batch")
  batchSync(@body body: BatchSyncRequest): BatchSyncResponse | ApiError;

  @doc("""
  List the caller's sightings, newest first, via keyset pagination. Pass the
  previous response's `nextCursor` as `cursor` to page. Soft-deleted rows are
  excluded.
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
  Enrich an existing sighting (full replace of the enrichable fields). Returns
  404 if the sighting does not exist or belongs to another user. Photo paths
  must be owned by the caller.
  """)
  @put
  update(@path id: SightingId, @body body: SightingUpdate): Sighting | ApiError;
}

@doc("The UK species reference list for offline caching.")
@route("/birds")
interface Birds {
  @doc("Return the full UK bird reference list. Response carries a strong ETag; changes approximately never. Supports conditional requests.")
  @get
  list(): Bird[] | ApiError;
}

@doc("The authenticated caller's account.")
@route("/me")
interface Account {
  @doc("Return the caller's profile, provisioning the user record on first call. Also the client's source of truth for the current tier.")
  @get
  get(): Me | ApiError;
}
