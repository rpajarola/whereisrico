// Package thin reduces dense GPS tracks (e.g. a Google Timeline export,
// which can have a point every few seconds) to a density the map frontend
// can render comfortably, and drops points inside excluded time windows
// such as flights.
package thin

import (
	"math"
	"sort"
	"time"

	"github.com/rpajarola/whereisrico/internal/storage"
)

// Options controls Thin.
type Options struct {
	// Interval: keep a point once at least this much time has passed since
	// the last kept point...
	Interval time.Duration
	// MaxDistanceMeters: ...or once it is at least this far from the last
	// kept point, so fast travel still gets enough points to follow the
	// route.
	MaxDistanceMeters float64
	// MinDistanceMeters: never keep a point closer than this to the last
	// kept point, regardless of time. This collapses long stationary
	// stretches (and the jitter of wifi/cell fixes while stationary) into
	// a single point.
	MinDistanceMeters float64
	// MaxSpeedKmh drops isolated outliers: a point is removed if both the
	// speed needed to get to it from the previous point and the speed
	// needed to get from it to the next point exceed this. Zero disables
	// the filter.
	MaxSpeedKmh float64
}

// Window is a time range whose interior (Start < t < End) is excluded.
// The endpoints themselves are kept, so e.g. a flight's departure and
// arrival airport points survive.
type Window struct {
	Start, End time.Time
}

// Thin returns a chronologically sorted subset of coords. Points inside
// any of exclude, and spikes as defined by MaxSpeedKmh, are dropped first.
// Of the rest, Thin keeps the first point, then each point that is at least
// MinDistanceMeters from the last kept point and either Interval after it
// or MaxDistanceMeters away from it.
// The final point is also kept if it moved at least MinDistanceMeters, so
// the track ends where the input ends. coords is not modified.
func Thin(coords []storage.Coord, opts Options, exclude []Window) []storage.Coord {
	sorted := make([]storage.Coord, 0, len(coords))
	for _, c := range coords {
		if !excluded(c.Timestamp, exclude) {
			sorted = append(sorted, c)
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Timestamp.Before(sorted[j].Timestamp) })
	if opts.MaxSpeedKmh > 0 {
		sorted = dropSpikes(sorted, opts.MaxSpeedKmh)
	}
	if len(sorted) == 0 {
		return nil
	}

	out := []storage.Coord{sorted[0]}
	for i, c := range sorted[1:] {
		last := out[len(out)-1]
		d := DistanceMeters(last.Latitude, last.Longitude, c.Latitude, c.Longitude)
		if d < opts.MinDistanceMeters {
			continue
		}
		isFinal := i == len(sorted)-2
		if isFinal || c.Timestamp.Sub(last.Timestamp) >= opts.Interval || d >= opts.MaxDistanceMeters {
			out = append(out, c)
		}
	}
	return out
}

// dropSpikes removes points that are only reachable at an impossible speed
// from both the last accepted point and the next point. Comparing against
// the last *accepted* point (rather than the previous raw one) keeps a
// glitch from also taking out the good point after it.
func dropSpikes(sorted []storage.Coord, maxSpeedKmh float64) []storage.Coord {
	out := make([]storage.Coord, 0, len(sorted))
	for i, c := range sorted {
		if len(out) > 0 && i+1 < len(sorted) &&
			speedKmh(out[len(out)-1], c) > maxSpeedKmh && speedKmh(c, sorted[i+1]) > maxSpeedKmh {
			continue
		}
		out = append(out, c)
	}
	return out
}

// speedKmh returns the speed needed to travel between a and b. Time
// differences below a minute are rounded up to one, so that position
// jitter between near-simultaneous fixes doesn't look like high speed.
func speedKmh(a, b storage.Coord) float64 {
	dt := b.Timestamp.Sub(a.Timestamp)
	if dt < 0 {
		dt = -dt
	}
	dt = max(dt, time.Minute)
	return DistanceMeters(a.Latitude, a.Longitude, b.Latitude, b.Longitude) / 1000 / dt.Hours()
}

func excluded(t time.Time, windows []Window) bool {
	for _, w := range windows {
		if t.After(w.Start) && t.Before(w.End) {
			return true
		}
	}
	return false
}

// DistanceMeters returns the great-circle (haversine) distance between two
// points.
func DistanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusMeters = 6371000
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLon := (lon2 - lon1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadiusMeters * math.Asin(math.Min(1, math.Sqrt(a)))
}
