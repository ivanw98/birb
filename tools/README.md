# tools

Build-time helpers.

## `gen-seed`: UK bird reference seed

Generates `db/migrations/00002_seed_birds.sql` from `uk_birds.tsv`.

```sh
go run ./tools/gen-seed   # from the repo root
```

A pure passthrough: validates ids (pattern + uniqueness) and eBird-code uniqueness,
assigns `taxonomic_order` from row position, emits the INSERT. It does not generate ids.

### `uk_birds.tsv`

Columns: `id`, `common_name`, `scientific_name`, `ebird_code`. BOU British List order,
which becomes `taxonomic_order` (list position, not the eBird global taxon order).

- **`id` is frozen.** Sightings reference these ids. Append new species with a fresh id;
  never renumber the column.
- ~627 of 642 species have an `ebird_code`. The rest are BOU rarities whose scientific
  names differ from eBird/Clements by gender or spelling (`Regulus ignicapillus` vs
  `ignicapilla`) and are left blank.

Built once from the BOU British List on
[Wikipedia](https://en.wikipedia.org/wiki/List_of_birds_of_Great_Britain), excluding the
"Species awaiting acceptance" appendix, joined to the
[eBird/Clements taxonomy](https://api.ebird.org/v2/ref/taxonomy/ebird?fmt=csv&cat=species)
for `ebird_code`, then one `brd_` id minted per row. Hand-edited since; treat it as
canonical.

## `gen-places`: UK populated-place seed

Generates `db/migrations/00006_seed_places.sql` from `uk_places.tsv`.

```sh
go run ./tools/gen-places   # from the repo root
```

A pure passthrough like `gen-seed`: validates id and `geonames_id` uniqueness, emits the
INSERT. It never mints ids and never touches the network.

The feed uses this table to turn a sighting's coordinates into "near \<somewhere\>". The
coordinates themselves never leave the server.

### `uk_places.tsv`

Columns: `id`, `geonames_id`, `name`, `latitude`, `longitude`, `population`,
`feature_code`. Sorted by `geonames_id` so a refresh diff reads as appends. Row order
means nothing to the database.

- **`id` is frozen.** Sightings resolve to a place; a renumbered id relabels history.
- **`geonames_id` is the natural key.** GeoNames preserves it across renames, coordinate
  fixes and reclassifications, so a refresh updates a row in place. It is `UNIQUE` in the
  database, so a renumbering bug fails the migration instead of duplicating a place.
- A place dropped from a later dump is kept, not deleted; its id may already be referenced.
- `population` is stored, not just filtered on. The feed floors on it, so changing how
  precisely a place name locates someone is a query change, not a reseed.

### Attribution: GeoNames, CC BY 4.0

`uk_places.tsv` derives from the [GeoNames](https://www.geonames.org/) GB dump
(<https://download.geonames.org/export/dump/GB.zip>), licensed
[CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). Attribution is a licence
condition: anything rendering these names must credit GeoNames.

### Refreshing

`tools/refresh-places` is the only thing that mints a `plc_` id. First build and later
refreshes are the same command.

```sh
curl -O https://download.geonames.org/export/dump/GB.zip
unzip -o GB.zip GB.txt
go run ./tools/refresh-places -dump GB.txt   # carries ids forward, mints for new geonames_ids
go run ./tools/gen-places                    # regenerates 00006
```

It takes a local path, not a URL, so the output does not depend on the day it ran.

Filter: feature class `P`, `population >= 500`, excluding `PPLH`, `PPLQ`, `PPLW` and
`PPLCH`. The floor is a privacy floor: below it a "near \<place\>" label can re-identify a
private location. The exclusions match nothing today and guard against GeoNames later
giving a historical site a population; an exclude-list means an unfamiliar new code shows
up in review rather than vanishing.

Stores `name` (dump column 1), not `asciiname` (column 2). They differ on one row today
(`Bo'ness`), but `asciiname` mangles Welsh and Gaelic once the list grows past GB.

Review a refresh by checking the diff is append-only in the `id` column. A `plc_` id that
disappears is a bug.

## ⚠️ Changing a seed after its migration has shipped

Both generators rewrite their migration in place. Goose records applied versions in
`goose_db_version` and never re-runs one, so a regenerated file only reaches databases
that have not applied it yet.

- Not applied anywhere yet: edit the TSV and regenerate.
- Already shipped: add a new migration with its own
  `INSERT ... ON CONFLICT (id) DO NOTHING` for new rows and explicit `UPDATE`s for
  corrections.
