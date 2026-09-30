package storage

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Leg is a [Start, End) timestamp bucket for one trip. Coordinates in this
// range belong to the leg, mirroring the old system's GetTrips() bucketing:
// consecutive trip start timestamps define bucket boundaries, with a
// synthesized leading unnamed leg (Name == "") if coordinates predate the
// first named trip, and a sentinel End one second past the last
// coordinate.
type Leg struct {
	Name       string
	Start, End time.Time
}

// Legs returns the trip-leg boundaries covering every coordinate strictly
// before the given instant. It returns a nil slice (no error) if there are
// no coordinates at all.
func (db *DB) Legs(ctx context.Context, before time.Time) ([]Leg, error) {
	minTS, maxTS, err := db.TimeRange(ctx, before)
	if err != nil {
		if errors.Is(err, ErrNoCoords) {
			return nil, nil
		}
		return nil, fmt.Errorf("storage: legs: %w", err)
	}

	trips, err := db.Trips(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage: legs: %w", err)
	}

	bounds := make([]time.Time, 0, len(trips)+2)
	names := make([]string, 0, len(trips)+1)
	if len(trips) == 0 || trips[0].Timestamp.After(minTS) {
		bounds = append(bounds, minTS)
		names = append(names, "")
	}
	for _, t := range trips {
		bounds = append(bounds, t.Timestamp)
		names = append(names, t.Name)
	}
	bounds = append(bounds, maxTS.Add(time.Second)) // sentinel end, exclusive

	legs := make([]Leg, len(bounds)-1)
	for i := range legs {
		legs[i] = Leg{Name: names[i], Start: bounds[i], End: bounds[i+1]}
	}
	return legs, nil
}
