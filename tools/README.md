# tools

Build-time helpers. Not part of the running service.

## `gen-seed` — UK bird reference seed

Generates `db/migrations/00002_seed_birds.sql` from `uk_birds.tsv`.

```sh
go run ./tools/gen-seed   # run from the repo root
```

`gen-seed` is a **pure passthrough**: it reads the TSV, validates the ids
(pattern + uniqueness) and eBird-code uniqueness, assigns `taxonomic_order` from
row position, and emits the INSERT. It does **not** generate ids.

### `uk_birds.tsv` — the committed source of truth

Tab-separated, with a header row: `id`, `common_name`, `scientific_name`,
`ebird_code`. Species are in BOU British List order (row order = the
`taxonomic_order` written to the DB — this is *list position*, not the eBird
global taxon order).

- **`id` is frozen.** Each `brd_` id was minted **once** and is now immutable
  data. This is the whole point: sightings reference these ids, so they must not
  move. Adding a species means appending a row with a fresh, never-before-used
  id. Correcting a name or code touches only that row. Never renumber or
  regenerate the id column — that would orphan existing references.
- ~627 of 642 species carry an `ebird_code`; the rest are BOU rarities whose
  scientific names differ from the eBird/Clements taxonomy by gender/spelling
  (e.g. `Regulus ignicapillus` vs eBird `ignicapilla`) and are left blank.

### How the TSV was built (one time)

1. Parsed the BOU British List from Wikipedia's
   [*List of birds of Great Britain*](https://en.wikipedia.org/wiki/List_of_birds_of_Great_Britain)
   (per-family `wikitable`s, taxonomic order preserved). The
   **"Species awaiting acceptance"** appendix is excluded — the seed is the
   accepted British List only.
2. Joined scientific names against the
   [eBird/Clements taxonomy](https://api.ebird.org/v2/ref/taxonomy/ebird?fmt=csv&cat=species)
   for `ebird_code`.
3. Minted a frozen `brd_` id per row.

Thereafter the TSV is edited by hand; treat it as canonical.

## ⚠️ Changing the list after 00002 has shipped

`gen-seed` rewrites `00002_seed_birds.sql` **in place**. Goose records applied
migrations in `goose_db_version` and **never re-runs** one (there is no
checksum), so regenerating `00002` only affects databases that have not yet
applied it (fresh CI/dev). On any database already migrated to version 2, a
regenerated `00002` is silently skipped.

Therefore:

- **Pre-release / not yet applied anywhere:** editing `uk_birds.tsv` and
  regenerating `00002` is fine.
- **After `00002` has shipped:** add the change as a **new** migration
  (`00003_...`) with its own `INSERT ... ON CONFLICT (id) DO NOTHING` for new
  rows and explicit `UPDATE`s for corrections. Do not rely on editing `00002`.
