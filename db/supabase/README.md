# Supabase Storage — `sighting-photos`

Photos are uploaded **directly from the browser** to Supabase Storage using the
user's own JWT (supabase-js), never proxied through the Go API. The API only
ever stores and returns bucket-relative object paths (`photo_paths`), and photo
paths are attached **only** via `PUT /api/sightings/{id}` — the batch sync
endpoint does not accept them.

These policies are **not** Goose migrations. On hosted Supabase the `postgres`
role no longer owns `storage.objects`, so `CREATE POLICY` on it fails when run
through Goose. Configure them once via the dashboard (Storage → Policies) or the
Supabase CLI. The reference SQL is kept here so the intended access model is
reviewable and reproducible.

## Bucket

- **Name:** `sighting-photos`
- **Public:** No (private). The client resolves short-lived signed URLs at
  display time.

## Canonical object path

```
<auth_uid>/<sighting_id>/<filename>
```

`<auth_uid>` is the Supabase `auth.users` UUID (matched by `auth.uid()` in the
policy), **not** the `usr_` application id. `<sighting_id>` is the `sgh_` id.
The first path segment is what the policy checks, so a user can only ever write
under their own prefix.

## Policies (reference SQL)

```sql
-- INSERT: users may upload only under their own uid prefix.
create policy "own-folder insert"
on storage.objects for insert to authenticated
with check (
  bucket_id = 'sighting-photos'
  and (storage.foldername(name))[1] = auth.uid()::text
);

-- SELECT: users may read only their own objects (used to mint signed URLs).
create policy "own-folder select"
on storage.objects for select to authenticated
using (
  bucket_id = 'sighting-photos'
  and (storage.foldername(name))[1] = auth.uid()::text
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
submitted photo path matches
`^<auth_uid>/<sighting_id>/[A-Za-z0-9._-]+\.(jpe?g|png|webp|heic)$`, so a caller
cannot attach another user's (or another sighting's) objects even though the
API uses a privileged connection.
