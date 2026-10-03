package main

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	exportgpx "github.com/rpajarola/whereisrico/internal/export/gpx"
	"github.com/rpajarola/whereisrico/internal/ingest/googletimeline"
	"github.com/rpajarola/whereisrico/internal/ingest/trip"
	"github.com/rpajarola/whereisrico/internal/storage"
	"github.com/rpajarola/whereisrico/internal/thin"
)

func runConvertTimeline(args []string) error {
	fs := flag.NewFlagSet("convert-timeline", flag.ExitOnError)
	in := fs.String("in", "", "path to the Google Maps Timeline export (Timeline.json)")
	out := fs.String("out", "", "path of the .gpx file to write (default with -trip: data/gpx/timeline-<trip>.gpx)")
	name := fs.String("name", "", "track name to put in the GPX file (default with -trip: the trip name)")
	tripName := fs.String("trip", "", "take the date range from this trip in -db, and drop points during the trip's flights")
	dbPath := fs.String("db", "data/whereisrico.db", "path to the sqlite database (only used with -trip)")
	from := fs.String("from", "", "only include points at or after this time (YYYY-MM-DD in UTC, or RFC 3339); overrides the trip's start")
	to := fs.String("to", "", "only include points up to this time; a YYYY-MM-DD date includes that whole day (UTC), an RFC 3339 time is exclusive; overrides the trip's end")
	raw := fs.Bool("raw", true, "include raw position fixes (rawSignals), not just the semantic timeline")
	maxAcc := fs.Float64("max-accuracy", 500, "drop raw position fixes with an accuracy radius above this many meters (0 = no limit)")
	noThin := fs.Bool("no-thin", false, "keep every point instead of thinning the track (flights and -max-speed outliers are still dropped)")
	interval := fs.Duration("interval", 20*time.Minute, "thinning: keep a point at least this often while moving")
	maxDist := fs.Float64("max-distance", 20000, "thinning: keep a point at least every this many meters while moving")
	minDist := fs.Float64("min-distance", 500, "thinning: drop points closer than this many meters to the last kept one (collapses stationary periods)")
	maxSpeed := fs.Float64("max-speed", 1200, "drop isolated points that would need more than this many km/h to reach and to leave (0 = keep them)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" {
		return fmt.Errorf("convert-timeline: -in is required")
	}

	opts := googletimeline.Options{IncludeRaw: *raw, MaxAccuracyMeters: *maxAcc}
	var exclude []thin.Window
	if *tripName != "" {
		var err error
		opts.From, opts.To, exclude, err = tripWindow(*dbPath, *tripName)
		if err != nil {
			return fmt.Errorf("convert-timeline: %w", err)
		}
		if *out == "" {
			*out = filepath.Join("data", "gpx", "timeline-"+exportSlugify(*tripName)+".gpx")
		}
		if *name == "" {
			*name = *tripName
		}
	}
	if *out == "" {
		return fmt.Errorf("convert-timeline: -out is required without -trip")
	}

	if *from != "" {
		t, err := parseTimeFlag(*from, false)
		if err != nil {
			return fmt.Errorf("convert-timeline: -from: %w", err)
		}
		opts.From = t
	}
	if *to != "" {
		t, err := parseTimeFlag(*to, true)
		if err != nil {
			return fmt.Errorf("convert-timeline: -to: %w", err)
		}
		opts.To = t
	}
	if *tripName != "" {
		fmt.Printf("trip %q: %s to %s, %d flight(s) excluded\n", *tripName,
			opts.From.Format(time.RFC3339), opts.To.Format(time.RFC3339), len(exclude))
	}

	coords, err := googletimeline.ParseFile(*in, opts)
	if err != nil {
		return fmt.Errorf("convert-timeline: %w", err)
	}
	parsed := len(coords)
	if !*noThin {
		coords = thin.Thin(coords, thin.Options{
			Interval:          *interval,
			MaxDistanceMeters: *maxDist,
			MinDistanceMeters: *minDist,
			MaxSpeedKmh:       *maxSpeed,
		}, exclude)
	} else {
		coords = thin.Thin(coords, thin.Options{MaxSpeedKmh: *maxSpeed}, exclude)
	}
	if len(coords) == 0 {
		return fmt.Errorf("convert-timeline: no points in %s match the given filters", *in)
	}
	if err := exportgpx.WriteFile(*out, *name, coords); err != nil {
		return fmt.Errorf("convert-timeline: %w", err)
	}
	fmt.Printf("wrote %s (%d of %d points)\n", *out, len(coords), parsed)
	return nil
}

// tripWindow looks up the named trip's time range and flights in the
// database. The range starts at the trip's start and ends just after the
// last messaged coordinate of its leg (usually the flight home's arrival),
// rather than at the next trip's start, since the time at home between
// trips is not part of the trip. A leg with no messaged coordinates ends at
// the next trip's start.
func tripWindow(dbPath, name string) (from, to time.Time, flights []thin.Window, err error) {
	db, err := storage.Open(dbPath)
	if err != nil {
		return from, to, nil, fmt.Errorf("open db: %w", err)
	}
	defer db.Close()

	ctx := context.Background()
	legs, err := db.Legs(ctx, time.Now())
	if err != nil {
		return from, to, nil, err
	}
	var leg *storage.Leg
	for i := range legs {
		if legs[i].Name == name {
			leg = &legs[i]
			break
		}
	}
	if leg == nil {
		return from, to, nil, fmt.Errorf("no trip named %q in %s", name, dbPath)
	}

	msgs, err := db.CoordsWithMessageBetween(ctx, leg.Start, leg.End)
	if err != nil {
		return from, to, nil, err
	}
	from, to = leg.Start, leg.End
	if len(msgs) > 0 {
		to = msgs[len(msgs)-1].Timestamp.Add(time.Second)
	}
	return from, to, flightWindows(msgs), nil
}

// flightWindows pairs each flight origin coordinate with the next
// coordinate whose message is its destination airport.
func flightWindows(msgs []storage.Coord) []thin.Window {
	var out []thin.Window
	for i, c := range msgs {
		_, dest, ok := trip.ParseFlightOrigin(c.Message)
		if !ok {
			continue
		}
		for _, d := range msgs[i+1:] {
			if d.Message == dest {
				out = append(out, thin.Window{Start: c.Timestamp, End: d.Timestamp})
				break
			}
		}
	}
	return out
}

// parseTimeFlag parses a -from/-to value: a YYYY-MM-DD date at UTC
// midnight (the following midnight if endOfDay), or an RFC 3339 timestamp.
func parseTimeFlag(s string, endOfDay bool) (time.Time, error) {
	if !strings.Contains(s, "T") {
		d, err := time.Parse(time.DateOnly, s)
		if err != nil {
			return time.Time{}, err
		}
		if endOfDay {
			d = d.AddDate(0, 0, 1)
		}
		return d, nil
	}
	return time.Parse(time.RFC3339, s)
}
