// Command gen-seed produces db/migrations/00002_seed_birds.sql from the frozen
// UK species list in tools/uk_birds.tsv.
//
// It is a build-time tool, not part of the running service. Run it from the
// repository root:
//
//	go run ./tools/gen-seed
//
// Each bird is assigned a deterministic, frozen `brd_` ULID: the value is a
// function only of the species' scientific name (for the 80-bit entropy) and
// its position in the list (for the 48-bit timestamp), so re-running the tool
// reproduces byte-identical SQL and the primary keys never move. That matters
// because sightings reference these ids.
//
// tools/uk_birds.tsv is the committed source of truth for the species list. It
// was generated once by joining the BOU British List (Wikipedia's "List of
// birds of Great Britain", in taxonomic order) against the eBird/Clements
// taxonomy for species codes; regenerating it is documented in tools/README.md.
package main

import (
	"bufio"
	"fmt"
	"math/big"
	"os"
	"strings"
)

// crockford is the lowercase Crockford base32 alphabet used by ULIDs. Every
// character is within [a-z0-9], satisfying the brd_ id CHECK constraint.
const crockford = "0123456789abcdefghjkmnpqrstvwxyz"

// baseTimestampMillis is a fixed epoch-milliseconds value (2026-07-11T00:00:00Z)
// used as the ULID timestamp base. It is constant so generation is
// deterministic; the per-row offset keeps the ids k-sortable in list order.
const baseTimestampMillis = 1_783_900_800_000

const (
	inputPath  = "tools/uk_birds.tsv"
	outputPath = "db/migrations/00002_seed_birds.sql"
)

type bird struct {
	id             string
	commonName     string
	scientificName string
	ebirdCode      string
	taxonomicOrder int
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gen-seed:", err)
		os.Exit(1)
	}
}

func run() error {
	birds, err := readBirds(inputPath)
	if err != nil {
		return err
	}
	if len(birds) == 0 {
		return fmt.Errorf("no birds parsed from %s", inputPath)
	}

	seen := make(map[string]string, len(birds))
	for i := range birds {
		birds[i].taxonomicOrder = i + 1
		birds[i].id = birdID(uint64(baseTimestampMillis+i), birds[i].scientificName)
		if prev, dup := seen[birds[i].id]; dup {
			return fmt.Errorf("duplicate generated id %s for %q and %q", birds[i].id, prev, birds[i].scientificName)
		}
		seen[birds[i].id] = birds[i].scientificName
	}

	sql := renderSQL(birds)
	if err := os.WriteFile(outputPath, []byte(sql), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d species)\n", outputPath, len(birds))
	return nil
}

func readBirds(path string) ([]bird, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var birds []bird
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first { // skip header row
			first = false
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 2 {
			return nil, fmt.Errorf("malformed row: %q", line)
		}
		b := bird{
			commonName:     strings.TrimSpace(cols[0]),
			scientificName: strings.TrimSpace(cols[1]),
		}
		if len(cols) >= 3 {
			b.ebirdCode = strings.TrimSpace(cols[2])
		}
		birds = append(birds, b)
	}
	return birds, sc.Err()
}

// birdID builds a frozen brd_-prefixed lowercase ULID from a timestamp and the
// species' scientific name. The 48-bit timestamp gives k-sortability; the
// 80-bit entropy is a stable digest of the name so the id never changes.
func birdID(tsMillis uint64, scientificName string) string {
	entropy := fnv80(scientificName)

	// value = (ts48 << 80) | rand80, encoded big-endian into 26 base32 chars.
	value := new(big.Int).SetUint64(tsMillis & ((1 << 48) - 1))
	value.Lsh(value, 80)
	value.Or(value, entropy)

	return "brd_" + base32ULID(value)
}

// fnv80 returns a deterministic 80-bit value derived from s using a 64-bit FNV-1a
// hash mixed into an 80-bit big.Int. Collisions across ~650 short strings are
// astronomically unlikely, and run() asserts uniqueness regardless.
func fnv80(s string) *big.Int {
	const (
		offset64 = 1469598103934665603
		prime64  = 1099511628211
	)
	var h uint64 = offset64
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}
	// Spread the 64-bit hash across 80 bits by folding in a second-pass hash of
	// the reversed bytes into the top 16 bits.
	var h2 uint64 = offset64
	for i := len(s) - 1; i >= 0; i-- {
		h2 ^= uint64(s[i])
		h2 *= prime64
	}
	top16 := (h2 >> 48) & 0xFFFF

	v := new(big.Int).SetUint64(top16)
	v.Lsh(v, 64)
	v.Or(v, new(big.Int).SetUint64(h))
	return v
}

// base32ULID encodes value as exactly 26 lowercase Crockford base32 characters,
// most-significant digit first, left-padded with '0'.
func base32ULID(value *big.Int) string {
	const n = 26
	out := make([]byte, n)
	v := new(big.Int).Set(value)
	mod := new(big.Int)
	base := big.NewInt(32)
	for i := n - 1; i >= 0; i-- {
		v.DivMod(v, base, mod)
		out[i] = crockford[mod.Int64()]
	}
	return string(out)
}

func renderSQL(birds []bird) string {
	var b strings.Builder
	b.WriteString("-- Code generated by tools/gen-seed; DO NOT EDIT.\n")
	b.WriteString("-- Source of truth: tools/uk_birds.tsv (regenerate via `go run ./tools/gen-seed`).\n")
	b.WriteString("-- UK bird reference list (BOU British List), taxonomic order preserved.\n\n")
	b.WriteString("-- +goose Up\n")
	b.WriteString("INSERT INTO birds (id, common_name, scientific_name, ebird_code, taxonomic_order) VALUES\n")

	for i, bd := range birds {
		sep := ","
		if i == len(birds)-1 {
			sep = ""
		}
		fmt.Fprintf(&b, "  ('%s', '%s', '%s', %s, %d)%s\n",
			bd.id,
			sqlQuote(bd.commonName),
			sqlQuote(bd.scientificName),
			sqlNullableText(bd.ebirdCode),
			bd.taxonomicOrder,
			sep,
		)
	}
	// Frozen ids mean a re-run re-uses the same primary keys; DO NOTHING makes
	// re-applying the seed a harmless no-op (dump/restore, manual psql runs,
	// version-table drift) rather than a primary-key crash. It deliberately
	// does not UPDATE — changing the list is a new migration, not a re-run.
	b.WriteString("ON CONFLICT (id) DO NOTHING;\n")

	b.WriteString("\n-- +goose Down\n")
	b.WriteString("-- No-op: migration 00001 drops the birds table, which removes these rows.\n")
	b.WriteString("-- A DELETE here would violate the sightings.bird_id foreign key when sightings exist.\n")
	b.WriteString("SELECT 1;\n")
	return b.String()
}

func sqlQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func sqlNullableText(s string) string {
	if s == "" {
		return "NULL"
	}
	return "'" + sqlQuote(s) + "'"
}
