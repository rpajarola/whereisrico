// Package trip loads hand-authored .textproto trip logs (see
// proto/trip/v1/trip.proto) into storage.Trip / storage.Coord values,
// replacing the old brittle tab-delimited CSX format. Unlike CSX, a typo or
// malformed line is a parse-time error here, not silently dropped data.
package trip

import (
	"fmt"
	"os"

	"google.golang.org/protobuf/encoding/prototext"

	tripv1 "github.com/rpajarola/whereisrico/internal/gen/trip/v1"
	"github.com/rpajarola/whereisrico/internal/geo/airport"
	"github.com/rpajarola/whereisrico/internal/geo/tz"
	"github.com/rpajarola/whereisrico/internal/storage"
)

// Loader parses .textproto trip logs into storage rows, resolving Flight
// entries via Airports and TZ.
type Loader struct {
	Airports airport.Lookup
	TZ       tz.Lookup
}

// LoadResult holds the trips and coordinates produced by loading one trip
// log. Leg membership of a Coord is not recorded here -- as in the old
// system, it is inferred later at query time from timestamp ranges against
// the trips table.
type LoadResult struct {
	Trips  []storage.Trip
	Coords []storage.Coord
}

// LoadFile reads and parses the .textproto file at path.
func (l *Loader) LoadFile(path string) (LoadResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LoadResult{}, fmt.Errorf("trip: read %s: %w", path, err)
	}

	var log tripv1.TripLog
	if err := prototext.Unmarshal(data, &log); err != nil {
		return LoadResult{}, fmt.Errorf("trip: parse %s: %w", path, err)
	}

	result, err := l.Load(path, &log)
	if err != nil {
		return LoadResult{}, fmt.Errorf("trip: %s: %w", path, err)
	}
	return result, nil
}

// Load converts an already-parsed TripLog into storage rows. source is
// recorded as the provenance of every produced row.
func (l *Loader) Load(source string, log *tripv1.TripLog) (LoadResult, error) {
	var result LoadResult
	for i, entry := range log.GetEntries() {
		switch e := entry.GetEntry().(type) {
		case *tripv1.Entry_TripStart:
			t, err := l.loadTripStart(source, e.TripStart)
			if err != nil {
				return LoadResult{}, fmt.Errorf("entry %d: %w", i, err)
			}
			result.Trips = append(result.Trips, t)

		case *tripv1.Entry_Waypoint:
			c, err := l.loadWaypoint(source, e.Waypoint)
			if err != nil {
				return LoadResult{}, fmt.Errorf("entry %d: %w", i, err)
			}
			result.Coords = append(result.Coords, c)

		case *tripv1.Entry_Flight:
			origin, dest, err := expandFlight(source, e.Flight, l.Airports, l.TZ)
			if err != nil {
				return LoadResult{}, fmt.Errorf("entry %d: %w", i, err)
			}
			result.Coords = append(result.Coords, origin, dest)

		default:
			return LoadResult{}, fmt.Errorf("entry %d: has no trip_start, waypoint, or flight set", i)
		}
	}
	return result, nil
}

func (l *Loader) loadTripStart(source string, ts *tripv1.TripStart) (storage.Trip, error) {
	if ts.GetName() == "" {
		return storage.Trip{}, fmt.Errorf("trip_start: name is required")
	}
	t, err := resolveTimeSpec(ts.GetTime(), false, 0, 0, l.TZ)
	if err != nil {
		return storage.Trip{}, fmt.Errorf("trip_start %q: %w", ts.GetName(), err)
	}
	return storage.Trip{Timestamp: t, Name: ts.GetName(), Source: source}, nil
}

func (l *Loader) loadWaypoint(source string, w *tripv1.Waypoint) (storage.Coord, error) {
	t, err := resolveTimeSpec(w.GetTime(), true, w.GetLatitude(), w.GetLongitude(), l.TZ)
	if err != nil {
		return storage.Coord{}, fmt.Errorf("waypoint: %w", err)
	}
	return storage.Coord{
		Timestamp: t,
		Message:   w.GetMessage(),
		Longitude: w.GetLongitude(),
		Latitude:  w.GetLatitude(),
		Source:    source,
	}, nil
}
