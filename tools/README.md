# tools

Build-time helpers. Not part of the running service.

## `gen-seed` — UK bird reference seed

Generates `db/migrations/00002_seed_birds.sql` from `uk_birds.tsv`.

```sh
go run ./tools/gen-seed   # run from the repo root
```

Each species gets a **frozen** `brd_` ULID derived deterministically from its
scientific name, so re-running produces byte-identical SQL and primary keys
never move (sightings reference them). The generator asserts id uniqueness and
fails if two ids collide.

### `uk_birds.tsv` — the committed source of truth

Tab-separated: `common_name`, `scientific_name`, `ebird_code`. Species are in
BOU taxonomic order (order in the file = `taxonomic_order` in the DB).

It was produced once by:

1. Parsing the BOU British List from Wikipedia's
   [*List of birds of Great Britain*](https://en.wikipedia.org/wiki/List_of_birds_of_Great_Britain)
   (per-family `wikitable`s, taxonomic order preserved) for common + scientific
   names.
2. Joining scientific names against the
   [eBird/Clements taxonomy](https://api.ebird.org/v2/ref/taxonomy/ebird?fmt=csv&cat=species)
   to attach `ebird_code`.

~628 of ~643 species match eBird exactly; the rest are BOU rarities whose
scientific names differ from eBird by gender/spelling (e.g. `Regulus
ignicapillus` vs eBird `ignicapilla`) and are left with a NULL `ebird_code`.
The list is edited by hand thereafter — treat the TSV as the canonical list and
regenerate the SQL from it; do not hand-edit `00002_seed_birds.sql`.
