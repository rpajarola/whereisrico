// Package googletimeline parses the on-device Google Maps Timeline export
// (the "Timeline.json" produced by Settings > Location > Timeline > Export
// on Android/iOS since Timeline moved off Google's servers) into a flat
// list of timestamped coordinates.
//
// The export has three top-level sections; positions are taken from:
//
//   - semanticSegments[].timelinePath[]: the bulk of the recorded path, one
//     point per sample.
//   - semanticSegments[].visit: the place's location, emitted at both the
//     visit's start and end time.
//   - semanticSegments[].activity: the start/end location, emitted at the
//     activity's start/end time.
//   - rawSignals[].position: raw fixes (only the last few weeks are kept in
//     the export), optionally filtered by reported accuracy.
//
// timelineMemory segments, wifiScan/activityRecord raw signals and
// userLocationProfile carry no timestamped position and are ignored.
package googletimeline

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rpajarola/whereisrico/internal/storage"
)

// Source is the storage.Coord Source set on every parsed coordinate.
const Source = "google-timeline"

// Options controls which records Parse turns into coordinates.
type Options struct {
	// IncludeRaw includes rawSignals[].position fixes.
	IncludeRaw bool
	// MaxAccuracyMeters drops raw fixes whose reported accuracy radius is
	// larger than this. Zero means no limit. Fixes without a reported
	// accuracy are always kept.
	MaxAccuracyMeters float64
	// From and To, if non-zero, restrict output to coordinates with
	// From <= timestamp < To.
	From, To time.Time
}

type export struct {
	SemanticSegments []segment   `json:"semanticSegments"`
	RawSignals       []rawSignal `json:"rawSignals"`
}

type segment struct {
	StartTime    string      `json:"startTime"`
	EndTime      string      `json:"endTime"`
	TimelinePath []pathPoint `json:"timelinePath"`
	Visit        *visit      `json:"visit"`
	Activity     *activity   `json:"activity"`
}

type pathPoint struct {
	Point string `json:"point"`
	Time  string `json:"time"`
}

type visit struct {
	TopCandidate struct {
		PlaceLocation latLng `json:"placeLocation"`
	} `json:"topCandidate"`
}

type activity struct {
	Start latLng `json:"start"`
	End   latLng `json:"end"`
}

type latLng struct {
	LatLng string `json:"latLng"`
}

type rawSignal struct {
	Position *struct {
		LatLng         string   `json:"LatLng"`
		AccuracyMeters *float64 `json:"accuracyMeters"`
		Timestamp      string   `json:"timestamp"`
	} `json:"position"`
}

// ParseFile parses the Timeline export at path.
func ParseFile(path string, opts Options) ([]storage.Coord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("googletimeline: open %s: %w", path, err)
	}
	defer f.Close()
	coords, err := Parse(f, opts)
	if err != nil {
		return nil, fmt.Errorf("googletimeline: parse %s: %w", path, err)
	}
	return coords, nil
}

// Parse parses a Timeline export from r. The result is deduplicated on
// (timestamp, latitude, longitude) but not sorted.
func Parse(r io.Reader, opts Options) ([]storage.Coord, error) {
	var doc export
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return nil, fmt.Errorf("googletimeline: decode: %w", err)
	}

	type key struct {
		unix     int64
		lat, lon float64
	}
	seen := make(map[key]bool)
	var coords []storage.Coord
	add := func(ts, ll string) error {
		if ll == "" || ts == "" {
			return nil
		}
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			return fmt.Errorf("time %q: %w", ts, err)
		}
		if !opts.From.IsZero() && t.Before(opts.From) {
			return nil
		}
		if !opts.To.IsZero() && !t.Before(opts.To) {
			return nil
		}
		lat, lon, err := parseLatLng(ll)
		if err != nil {
			return err
		}
		k := key{t.Unix(), lat, lon}
		if seen[k] {
			return nil
		}
		seen[k] = true
		coords = append(coords, storage.Coord{Timestamp: t, Latitude: lat, Longitude: lon, Source: Source})
		return nil
	}

	for i, s := range doc.SemanticSegments {
		var err error
		for _, p := range s.TimelinePath {
			if err = add(p.Time, p.Point); err != nil {
				break
			}
		}
		if err == nil && s.Visit != nil {
			loc := s.Visit.TopCandidate.PlaceLocation.LatLng
			if err = add(s.StartTime, loc); err == nil {
				err = add(s.EndTime, loc)
			}
		}
		if err == nil && s.Activity != nil {
			if err = add(s.StartTime, s.Activity.Start.LatLng); err == nil {
				err = add(s.EndTime, s.Activity.End.LatLng)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("googletimeline: semanticSegments[%d]: %w", i, err)
		}
	}

	if opts.IncludeRaw {
		for i, s := range doc.RawSignals {
			p := s.Position
			if p == nil {
				continue
			}
			if opts.MaxAccuracyMeters > 0 && p.AccuracyMeters != nil && *p.AccuracyMeters > opts.MaxAccuracyMeters {
				continue
			}
			if err := add(p.Timestamp, p.LatLng); err != nil {
				return nil, fmt.Errorf("googletimeline: rawSignals[%d]: %w", i, err)
			}
		}
	}

	return coords, nil
}

// parseLatLng parses the export's "46.689618°, 7.7717974°" coordinate
// strings.
func parseLatLng(s string) (lat, lon float64, err error) {
	latStr, lonStr, ok := strings.Cut(s, ",")
	if !ok {
		return 0, 0, fmt.Errorf("latLng %q: missing comma", s)
	}
	clean := func(v string) string { return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "°")) }
	if lat, err = strconv.ParseFloat(clean(latStr), 64); err != nil {
		return 0, 0, fmt.Errorf("latLng %q: latitude: %w", s, err)
	}
	if lon, err = strconv.ParseFloat(clean(lonStr), 64); err != nil {
		return 0, 0, fmt.Errorf("latLng %q: longitude: %w", s, err)
	}
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return 0, 0, fmt.Errorf("latLng %q: out of range", s)
	}
	return lat, lon, nil
}
