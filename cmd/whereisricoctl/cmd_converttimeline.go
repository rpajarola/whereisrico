package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	exportgpx "github.com/rpajarola/whereisrico/internal/export/gpx"
	"github.com/rpajarola/whereisrico/internal/ingest/googletimeline"
)

func runConvertTimeline(args []string) error {
	fs := flag.NewFlagSet("convert-timeline", flag.ExitOnError)
	in := fs.String("in", "", "path to the Google Maps Timeline export (Timeline.json)")
	out := fs.String("out", "", "path of the .gpx file to write")
	name := fs.String("name", "", "track name to put in the GPX file")
	from := fs.String("from", "", "only include points at or after this time (YYYY-MM-DD in UTC, or RFC 3339)")
	to := fs.String("to", "", "only include points up to this time; a YYYY-MM-DD date includes that whole day (UTC), an RFC 3339 time is exclusive")
	raw := fs.Bool("raw", true, "include raw position fixes (rawSignals), not just the semantic timeline")
	maxAcc := fs.Float64("max-accuracy", 500, "drop raw position fixes with an accuracy radius above this many meters (0 = no limit)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" || *out == "" {
		return fmt.Errorf("convert-timeline: -in and -out are required")
	}

	opts := googletimeline.Options{IncludeRaw: *raw, MaxAccuracyMeters: *maxAcc}
	var err error
	if opts.From, err = parseTimeFlag(*from, false); err != nil {
		return fmt.Errorf("convert-timeline: -from: %w", err)
	}
	if opts.To, err = parseTimeFlag(*to, true); err != nil {
		return fmt.Errorf("convert-timeline: -to: %w", err)
	}

	coords, err := googletimeline.ParseFile(*in, opts)
	if err != nil {
		return fmt.Errorf("convert-timeline: %w", err)
	}
	if len(coords) == 0 {
		return fmt.Errorf("convert-timeline: no points in %s match the given filters", *in)
	}
	if err := exportgpx.WriteFile(*out, *name, coords); err != nil {
		return fmt.Errorf("convert-timeline: %w", err)
	}
	fmt.Printf("wrote %s (%d points)\n", *out, len(coords))
	return nil
}

// parseTimeFlag parses a -from/-to value: empty (zero time), a YYYY-MM-DD
// date at UTC midnight (the following midnight if endOfDay), or an RFC 3339
// timestamp.
func parseTimeFlag(s string, endOfDay bool) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
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
