// Package airport resolves an IATA airport code to its coordinates using a
// trimmed, embedded extract of the OpenFlights airport database -- entirely
// offline (no network calls, no API keys), replacing the old RapidAPI
// airport-info dependency. See ATTRIBUTION.md for data source and license.
package airport

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

//go:embed airports.csv
var airportsCSV string

// Airport is a single airport's identity and location.
type Airport struct {
	IATA      string
	Name      string
	City      string
	Country   string
	Latitude  float64
	Longitude float64
}

// Lookup resolves IATA airport codes to airports.
type Lookup interface {
	ByIATA(code string) (Airport, bool)
}

type table struct {
	byIATA map[string]Airport
}

var (
	defaultOnce  sync.Once
	defaultTable *table
	defaultErr   error
)

// New returns a Lookup backed by the embedded airport database. Parsing
// happens once per process; repeated calls return the same shared table.
func New() (Lookup, error) {
	defaultOnce.Do(func() {
		defaultTable, defaultErr = parse(airportsCSV)
	})
	if defaultErr != nil {
		return nil, defaultErr
	}
	return defaultTable, nil
}

func parse(data string) (*table, error) {
	r := csv.NewReader(strings.NewReader(data))
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("airport: parse embedded csv: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("airport: embedded csv is empty")
	}

	t := &table{byIATA: make(map[string]Airport, len(records)-1)}
	for _, rec := range records[1:] { // skip header row
		if len(rec) != 6 {
			return nil, fmt.Errorf("airport: malformed row %q", rec)
		}
		lat, err := strconv.ParseFloat(rec[4], 64)
		if err != nil {
			return nil, fmt.Errorf("airport: parse latitude in row %q: %w", rec, err)
		}
		lon, err := strconv.ParseFloat(rec[5], 64)
		if err != nil {
			return nil, fmt.Errorf("airport: parse longitude in row %q: %w", rec, err)
		}
		a := Airport{
			IATA:      strings.ToUpper(rec[0]),
			Name:      rec[1],
			City:      rec[2],
			Country:   rec[3],
			Latitude:  lat,
			Longitude: lon,
		}
		t.byIATA[a.IATA] = a
	}
	return t, nil
}

// ByIATA looks up an airport by its 3-letter IATA code (case-insensitive).
func (t *table) ByIATA(code string) (Airport, bool) {
	a, ok := t.byIATA[strings.ToUpper(strings.TrimSpace(code))]
	return a, ok
}
