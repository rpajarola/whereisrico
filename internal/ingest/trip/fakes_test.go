package trip

import (
	"fmt"

	"github.com/rpajarola/whereisrico/internal/geo/airport"
)

// fakeTZ always resolves to a single fixed zone, regardless of coordinates.
type fakeTZ struct {
	zone string
}

func (f fakeTZ) ZoneName(lat, lon float64) (string, error) {
	return f.zone, nil
}

// fakeZonedTZ resolves to different zones based on a coordinate match,
// falling back to an error for anything else -- lets tests assert that the
// *correct* coordinates were used to resolve a zone (e.g. origin airport's,
// not the destination's).
type fakeZonedTZ struct {
	byCoord map[[2]float64]string
}

func (f fakeZonedTZ) ZoneName(lat, lon float64) (string, error) {
	zone, ok := f.byCoord[[2]float64{lat, lon}]
	if !ok {
		return "", fmt.Errorf("fakeZonedTZ: no zone configured for (%v, %v)", lat, lon)
	}
	return zone, nil
}

type fakeAirports struct {
	byCode map[string]airport.Airport
}

func (f fakeAirports) ByIATA(code string) (airport.Airport, bool) {
	a, ok := f.byCode[code]
	return a, ok
}
