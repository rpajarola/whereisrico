// Package tz resolves an IANA timezone name from a latitude/longitude pair,
// entirely offline (no network calls, no API keys), replacing the old
// Google Maps Timezone API dependency.
package tz

import (
	"fmt"

	"github.com/ringsaturn/tzf"
)

// Lookup resolves geographic coordinates to an IANA timezone name.
type Lookup interface {
	ZoneName(lat, lon float64) (string, error)
}

type finder struct {
	f tzf.F
}

// New returns a Lookup backed by tzf's simplified ("lite") boundary
// dataset. Boundary precision is within ~111m of the true border, which is
// more than sufficient for resolving the timezone of a trip waypoint or
// airport -- this app has no need for the more expensive 100%-accurate
// tzf.NewFullFinder.
//
// Construction loads the boundary dataset into memory; callers should build
// one Lookup and reuse it rather than calling New repeatedly.
func New() (Lookup, error) {
	f, err := tzf.NewDefaultFinder()
	if err != nil {
		return nil, fmt.Errorf("tz: init finder: %w", err)
	}
	return &finder{f: f}, nil
}

// ZoneName returns the IANA timezone name (e.g. "Europe/Zurich") containing
// the given coordinates.
func (fd *finder) ZoneName(lat, lon float64) (string, error) {
	name := fd.f.GetTimezoneName(lon, lat) // tzf takes (lng, lat) order.
	if name == "" {
		return "", fmt.Errorf("tz: no timezone found for (%v, %v)", lat, lon)
	}
	return name, nil
}
