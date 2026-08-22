package main

// Command refresh-places builds tools/uk_places.tsv from a GeoNames country dump.
// Run from the repository root:
//
//	curl -O https://download.geonames.org/export/dump/GB.zip
//	unzip -o GB.zip GB.txt
//	go run ./tools/refresh-places -dump GB.txt
import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/oklog/ulid/v2"
)

const (
	tsvPath       = "tools/uk_places.tsv"
	header        = "id\tgeonames_id\tname\tlatitude\tlongitude\tpopulation\tfeature_code"
	minPopulation = 500
)

// GeoNames "geoname" table columns, 0-based, per the readme.txt bundled with the dump.
const (
	colGeonameID   = 0
	colName        = 1
	colLatitude    = 4
	colLongitude   = 5
	colFeatureCls  = 6
	colFeatureCode = 7
	colPopulation  = 14
	colCount       = 19
)

// excludedCodes drops places that no longer exist. None survive the population floor
// today; this is a guard for the day GeoNames attaches a population to one.
var excludedCodes = map[string]bool{
	"PPLH":  true, // historical
	"PPLQ":  true, // abandoned
	"PPLW":  true, // destroyed
	"PPLCH": true, // historical seat of an administrative division
}

var placeIDPattern = regexp.MustCompile(`^plc_[a-z0-9]{26}$`)

type place struct {
	id          string
	geonamesID  int
	name        string
	latitude    string
	longitude   string
	population  int
	featureCode string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "refresh-places:", err)
		os.Exit(1)
	}
}

func run() error {
	dump := flag.String("dump", "", "path to an unzipped GeoNames country dump, e.g. GB.txt")
	flag.Parse()
	if *dump == "" {
		return fmt.Errorf("-dump is required")
	}

	existing, err := readExisting(tsvPath)
	if err != nil {
		return err
	}

	dumped, err := readDump(*dump)
	if err != nil {
		return err
	}
	if len(dumped) == 0 {
		return fmt.Errorf("no places survived the filter in %s", *dump)
	}

	var minted, updated int
	for i, p := range dumped {
		prev, known := existing[p.geonamesID]
		if !known {
			dumped[i].id = "plc_" + strings.ToLower(ulid.Make().String())
			minted++
			continue
		}
		dumped[i].id = prev.id
		if prev != dumped[i] {
			updated++
		}
		delete(existing, p.geonamesID)
	}

	// Whatever the dump no longer lists stays, so a stored id never dangles.
	orphans := make([]place, 0, len(existing))
	for _, p := range existing {
		orphans = append(orphans, p)
	}
	out := append(dumped, orphans...)
	sort.Slice(out, func(i, j int) bool { return out[i].geonamesID < out[j].geonamesID })

	if err := os.WriteFile(tsvPath, []byte(renderTSV(out)), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d places: %d carried, %d minted, %d field-updated, %d no longer in the dump)\n",
		tsvPath, len(out), len(out)-minted-len(orphans), minted, updated, len(orphans))
	for _, p := range orphans {
		fmt.Printf("  kept, absent from dump: %s %s (geonames_id %d)\n", p.id, p.name, p.geonamesID)
	}
	return nil
}

// readExisting loads the committed TSV keyed by geonames_id. A missing file is the
// first run, not an error.
func readExisting(path string) (map[int]place, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return map[int]place{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	byGeonames := map[int]place{}
	seenID := map[string]string{}
	sc := bufio.NewScanner(f)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			first = false
			continue
		}
		if strings.TrimSpace(line) == "" {
			continue
		}

		p, err := parseTSVRow(line)
		if err != nil {
			return nil, err
		}
		if !placeIDPattern.MatchString(p.id) {
			return nil, fmt.Errorf("id %q (%s) does not match %s", p.id, p.name, placeIDPattern)
		}
		if prev, dup := seenID[p.id]; dup {
			return nil, fmt.Errorf("duplicate id %s shared by %q and %q", p.id, prev, p.name)
		}
		seenID[p.id] = p.name
		if prev, dup := byGeonames[p.geonamesID]; dup {
			return nil, fmt.Errorf("duplicate geonames_id %d shared by %q and %q", p.geonamesID, prev.name, p.name)
		}
		byGeonames[p.geonamesID] = p
	}
	return byGeonames, sc.Err()
}

func parseTSVRow(line string) (place, error) {
	f := strings.Split(line, "\t")
	if len(f) != 7 {
		return place{}, fmt.Errorf("expected 7 columns in %s, got %d: %q", tsvPath, len(f), line)
	}
	geonamesID, err := strconv.Atoi(f[1])
	if err != nil {
		return place{}, fmt.Errorf("geonames_id %q is not a number: %w", f[1], err)
	}
	population, err := strconv.Atoi(f[5])
	if err != nil {
		return place{}, fmt.Errorf("population %q is not a number: %w", f[5], err)
	}
	return place{
		id: f[0], geonamesID: geonamesID, name: f[2],
		latitude: f[3], longitude: f[4], population: population, featureCode: f[6],
	}, nil
}

// readDump parses the GeoNames dump, keeping populated places above the floor.
func readDump(path string) ([]place, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var out []place
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		cols := strings.Split(sc.Text(), "\t")
		if len(cols) != colCount {
			continue
		}
		if cols[colFeatureCls] != "P" || excludedCodes[cols[colFeatureCode]] {
			continue
		}
		population, err := strconv.Atoi(cols[colPopulation])
		if err != nil || population < minPopulation {
			continue
		}
		geonamesID, err := strconv.Atoi(cols[colGeonameID])
		if err != nil {
			return nil, fmt.Errorf("geonameid %q is not a number", cols[colGeonameID])
		}
		if strings.ContainsAny(cols[colName], "\t\n") {
			return nil, fmt.Errorf("name %q contains a tab or newline and would break the TSV", cols[colName])
		}

		out = append(out, place{
			geonamesID:  geonamesID,
			name:        cols[colName],
			latitude:    cols[colLatitude],
			longitude:   cols[colLongitude],
			population:  population,
			featureCode: cols[colFeatureCode],
		})
	}
	return out, sc.Err()
}

// renderTSV writes the file sorted by geonames_id, so a refresh diff reads as appends
// and in-place edits rather than a whole-file reshuffle.
func renderTSV(places []place) string {
	var b strings.Builder
	b.WriteString(header + "\n")
	for _, p := range places {
		fmt.Fprintf(&b, "%s\t%d\t%s\t%s\t%s\t%d\t%s\n",
			p.id, p.geonamesID, p.name, p.latitude, p.longitude, p.population, p.featureCode)
	}
	return b.String()
}
