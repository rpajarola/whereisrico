// Package geojson builds a GeoJSON FeatureCollection describing trip legs
// and annotated waypoints from the storage layer, for the frontend to
// render with MapLibre GL JS. It ports the old system's trip-bucketing and
// coloring logic (GetTrips/GetColor/AddMessages in whereisrico_lib.py) as
// server-computed facts; presentation (actual colors, opacity) is applied
// client-side via MapLibre paint expressions driven by these facts.
package geojson

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rpajarola/whereisrico/internal/storage"
)

const numColors = 5

// Options configures Build.
type Options struct {
	// Now is the reference instant used to determine which coordinate is
	// "here" (the most recent) and to exclude future-dated coordinates.
	// Defaults to time.Now() if zero.
	Now time.Time
}

// FeatureCollection is a minimal GeoJSON FeatureCollection with an
// additional "meta" member (a valid GeoJSON foreign member) carrying the
// overall timestamp range the client needs for recency-based styling.
type FeatureCollection struct {
	Type     string    `json:"type"`
	Meta     Meta      `json:"meta"`
	Features []Feature `json:"features"`
}

// Meta carries the facts a client needs to compute presentation (e.g. an
// opacity fade) that don't belong on any single feature.
type Meta struct {
	MinTimestamp int64 `json:"min_timestamp"`
	MaxTimestamp int64 `json:"max_timestamp"`
	GeneratedAt  int64 `json:"generated_at"`
}

// Feature is a GeoJSON Feature restricted to the geometry/property shapes
// this package produces (LineString trip legs, Point waypoints).
type Feature struct {
	Type       string     `json:"type"`
	Geometry   Geometry   `json:"geometry"`
	Properties Properties `json:"properties"`
}

// Geometry is a GeoJSON geometry: either a "Point" ([lon,lat]) or a
// "LineString" ([][lon,lat]). Only one of Point/LineString is populated,
// matching Type.
type Geometry struct {
	Type       string       `json:"type"`
	Point      [2]float64   `json:"-"`
	LineString [][2]float64 `json:"-"`
}

// Properties covers both "trip" (LineString) and "waypoint" (Point)
// features; fields not applicable to a given Kind are left zero/omitted.
type Properties struct {
	Kind           string `json:"kind"` // "trip" or "waypoint"
	TripIndex      int    `json:"trip_index"`
	ColorIndex     int    `json:"color_index"`
	IsLatestTrip   bool   `json:"is_latest_trip"`
	TripName       string `json:"trip_name,omitempty"`
	StartTimestamp int64  `json:"start_timestamp,omitempty"`
	EndTimestamp   int64  `json:"end_timestamp,omitempty"`
	Message        string `json:"message,omitempty"`
	Timestamp      int64  `json:"timestamp,omitempty"`
	// Not omitempty: unlike IsLatestTrip's sibling bool, this is false for
	// the overwhelming majority of waypoints, and a JSON consumer that
	// reads "is this property present" as its truthiness check (as
	// Cesium's PropertyBag does -- observed via web/static/cesium.js)
	// would otherwise treat every non-latest waypoint as if is_here were
	// simply unknown rather than explicitly false.
	IsHere bool `json:"is_here"`
}

// leg is a [start, end) timestamp bucket for one trip, mirroring the old
// GetTrips()'s bucketing: consecutive trip start timestamps define bucket
// boundaries, with a synthesized leading unnamed leg if coordinates predate
// the first named trip, and a sentinel end boundary past the last
// coordinate.
type leg struct {
	start, end time.Time
	name       string
}

// Build queries db and assembles the full GeoJSON FeatureCollection.
func Build(ctx context.Context, db *storage.DB, opts Options) (*FeatureCollection, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	minTS, maxTS, err := db.TimeRange(ctx, now)
	if err != nil {
		if errors.Is(err, storage.ErrNoCoords) {
			return &FeatureCollection{
				Type:     "FeatureCollection",
				Meta:     Meta{GeneratedAt: now.Unix()},
				Features: []Feature{},
			}, nil
		}
		return nil, fmt.Errorf("geojson: time range: %w", err)
	}

	trips, err := db.Trips(ctx)
	if err != nil {
		return nil, fmt.Errorf("geojson: trips: %w", err)
	}
	legs := buildLegs(trips, minTS, maxTS)

	fc := &FeatureCollection{
		Type: "FeatureCollection",
		Meta: Meta{
			MinTimestamp: minTS.Unix(),
			MaxTimestamp: maxTS.Unix(),
			GeneratedAt:  now.Unix(),
		},
	}

	for i, lg := range legs {
		isLatest := i == len(legs)-1
		colorIndex := i % numColors

		coords, err := db.CoordsBetween(ctx, lg.start, lg.end)
		if err != nil {
			return nil, fmt.Errorf("geojson: coords for leg %d: %w", i, err)
		}
		if len(coords) > 0 {
			line := make([][2]float64, len(coords))
			for j, c := range coords {
				line[j] = [2]float64{c.Longitude, c.Latitude}
			}
			fc.Features = append(fc.Features, Feature{
				Type:     "Feature",
				Geometry: Geometry{Type: "LineString", LineString: line},
				Properties: Properties{
					Kind:           "trip",
					TripIndex:      i,
					ColorIndex:     colorIndex,
					IsLatestTrip:   isLatest,
					TripName:       lg.name,
					StartTimestamp: lg.start.Unix(),
					EndTimestamp:   lg.end.Unix(),
				},
			})
		}

		messaged, err := db.CoordsWithMessageBetween(ctx, lg.start, lg.end)
		if err != nil {
			return nil, fmt.Errorf("geojson: messaged coords for leg %d: %w", i, err)
		}
		for _, c := range messaged {
			fc.Features = append(fc.Features, Feature{
				Type:     "Feature",
				Geometry: Geometry{Type: "Point", Point: [2]float64{c.Longitude, c.Latitude}},
				Properties: Properties{
					Kind:         "waypoint",
					TripIndex:    i,
					ColorIndex:   colorIndex,
					IsLatestTrip: isLatest,
					Message:      c.Message,
					Timestamp:    c.Timestamp.Unix(),
					IsHere:       c.Timestamp.Equal(maxTS),
				},
			})
		}
	}

	if !hasHereMarker(fc.Features) {
		here, err := hereFeature(ctx, db, maxTS, legs)
		if err != nil {
			return nil, fmt.Errorf("geojson: here marker: %w", err)
		}
		if here != nil {
			fc.Features = append(fc.Features, *here)
		}
	}

	if fc.Features == nil {
		fc.Features = []Feature{}
	}
	return fc, nil
}

// buildLegs turns the trips table into [start,end) buckets covering
// [min(coord ts), max(coord ts)]. If coordinates predate the first named
// trip, a leading unnamed leg is synthesized, matching the old
// GetTrips()'s behavior.
func buildLegs(trips []storage.Trip, minTS, maxTS time.Time) []leg {
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

	legs := make([]leg, len(bounds)-1)
	for i := range legs {
		legs[i] = leg{start: bounds[i], end: bounds[i+1], name: names[i]}
	}
	return legs
}

func hasHereMarker(features []Feature) bool {
	for _, f := range features {
		if f.Properties.IsHere {
			return true
		}
	}
	return false
}

// hereFeature synthesizes the "I am here" marker when the most recent
// coordinate has no message (so the loop above never emitted a waypoint
// feature for it), matching the old AddMessages()'s fallback branch.
func hereFeature(ctx context.Context, db *storage.DB, maxTS time.Time, legs []leg) (*Feature, error) {
	coords, err := db.CoordsBetween(ctx, maxTS, maxTS.Add(time.Second))
	if err != nil {
		return nil, err
	}
	if len(coords) == 0 {
		return nil, nil
	}
	c := coords[0]

	tripIndex := len(legs) - 1
	for i, lg := range legs {
		if !c.Timestamp.Before(lg.start) && c.Timestamp.Before(lg.end) {
			tripIndex = i
			break
		}
	}

	return &Feature{
		Type:     "Feature",
		Geometry: Geometry{Type: "Point", Point: [2]float64{c.Longitude, c.Latitude}},
		Properties: Properties{
			Kind:         "waypoint",
			TripIndex:    tripIndex,
			ColorIndex:   tripIndex % numColors,
			IsLatestTrip: tripIndex == len(legs)-1,
			Message:      "I am here",
			Timestamp:    c.Timestamp.Unix(),
			IsHere:       true,
		},
	}, nil
}
