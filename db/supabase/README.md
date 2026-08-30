# Supabase Storage: `sighting-photos` and `sighting-recordings`

Photos and recordings are uploaded **directly from the browser** to Supabase
Storage using the user's own JWT (supabase-js), never proxied through the Go
API. The API only ever stores and returns bucket-relative object paths
(`photo_paths`, `recording_paths`), and both are attached **only** via
`PUT /api/sightings/{id}`; the batch sync endpoint does not accept them.

These policies are **not** Goose migrations. On hosted Supabase the `postgres`
role no longer owns `storage.objects`, so `CREATE POLICY` on it fails when run
through Goose. Configure them once via the dashboard (Storage → Policies) or the
Supabase CLI. The reference SQL is kept here so the intended access model is
reviewable and reproducible.

`can_view_sighting_media(viewer_auth_id, object_name)`, used by the co-member
policies below, **is** a Goose migration (`00007_recordings.sql`) — it's a
plain SQL function with no `auth.*`/`storage.*` references, so unlike a
storage policy it needs no dashboard access and runs the same in CI.

## Buckets

- **Names:** `sighting-photos`, `sighting-recordings`
- **Public:** No (private) on both. The client resolves short-lived signed
  URLs at display time, whether the caller owns the object or is viewing a
  co-member's via the feed.

## File size limits

Photo and recording bytes upload directly from the browser to Storage (see
above) — the Go API only ever sees the resulting object path, never the file
itself. That makes the per-bucket file-size limit, set in the Supabase
dashboard (Storage → bucket → Settings), the **only** server-side
enforcement of upload size; there is no equivalent API-side check to fall
back on. Set one for both `sighting-photos` and `sighting-recordings` — 5 MB
per file is a reasonable cap for a compressed photo or a capped-length voice
recording.

## Canonical object path

```
<auth_uid>/<sighting_id>/<filename>
```

`<auth_uid>` is the Supabase `auth.users` UUID (matched by `auth.uid()` in the
policy), **not** the `usr_` application id. `<sighting_id>` is the `sgh_` id.
The first path segment is what the own-folder policies check, so a user can
only ever write under their own prefix. Same convention in both buckets.

## Policies (reference SQL)

Apply this block once per bucket, substituting `bucket_id`.

```sql
-- INSERT: users may upload only under their own uid prefix.
create policy "own-folder insert"
on storage.objects for insert to authenticated
with check (
  bucket_id = 'sighting-photos'  -- and again for 'sighting-recordings'
  and (storage.foldername(name))[1] = auth.uid()::text
);

-- SELECT: users may always read their own objects (used to mint signed URLs).
create policy "own-folder select"
on storage.objects for select to authenticated
using (
  bucket_id = 'sighting-photos'
  and (storage.foldername(name))[1] = auth.uid()::text
);

-- SELECT: users may also read a co-member's objects, for the social feed.
-- Membership (not the 7-day feed window) is the sharing boundary: a co-member's
-- media stays visible via signed URL after their sighting scrolls out of the feed.
create policy "co-member select"
on storage.objects for select to authenticated
using (
  bucket_id = 'sighting-photos'
  and can_view_sighting_media(auth.uid(), name)
);

-- DELETE: users may remove only their own objects.
create policy "own-folder delete"
on storage.objects for delete to authenticated
using (
  bucket_id = 'sighting-photos'
  and (storage.foldername(name))[1] = auth.uid()::text
);
```

The Go API additionally validates, on `PUT /api/sightings/{id}`, that every
submitted path matches
`^<auth_uid>/<sighting_id>/[A-Za-z0-9._-]+\.<ext>$` — photos:
`(jpe?g|png|webp|heic)`, recordings: `(webm|ogg|m4a|mp4)` — so a caller cannot
attach another user's (or another sighting's) objects even though the API
uses a privileged connection.
