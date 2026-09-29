package trip

import (
	"fmt"
	"time"

	// Embed the IANA timezone database into the binary so zone names like
	// "Europe/Zurich" or legacy POSIX-ish names like "PST8PDT" resolve
	// without depending on the host having a system zoneinfo database
	// installed (e.g. a minimal container image).
	_ "time/tzdata"

	tripv1 "github.com/rpajarola/whereisrico/internal/gen/trip/v1"
	"github.com/rpajarola/whereisrico/internal/geo/tz"
)

// resolveTimeSpec resolves a TimeSpec to a UTC instant. If the spec is a
// LocalDateTime with no explicit zone, hasCoords/lat/lon are used to resolve
// the zone offline via tzs.
func resolveTimeSpec(spec *tripv1.TimeSpec, hasCoords bool, lat, lon float64, tzs tz.Lookup) (time.Time, error) {
	if spec == nil {
		return time.Time{}, fmt.Errorf("time: missing TimeSpec")
	}
	switch v := spec.GetSpec().(type) {
	case *tripv1.TimeSpec_Absolute:
		return v.Absolute.AsTime().UTC(), nil
	case *tripv1.TimeSpec_Local:
		return resolveLocalDateTime(v.Local, hasCoords, lat, lon, tzs)
	default:
		return time.Time{}, fmt.Errorf("time: TimeSpec has neither absolute nor local set")
	}
}

func resolveLocalDateTime(l *tripv1.LocalDateTime, hasCoords bool, lat, lon float64, tzs tz.Lookup) (time.Time, error) {
	if l.GetDate() == "" {
		return time.Time{}, fmt.Errorf("time: LocalDateTime.date is required")
	}
	clock := l.GetTime()
	if clock == "" {
		clock = "00:00"
	}

	zoneName := l.GetZone()
	if zoneName == "" {
		if !hasCoords {
			return time.Time{}, fmt.Errorf("time: LocalDateTime has no zone and no coordinates to resolve one from")
		}
		var err error
		zoneName, err = tzs.ZoneName(lat, lon)
		if err != nil {
			return time.Time{}, fmt.Errorf("time: resolve zone from coordinates (%v, %v): %w", lat, lon, err)
		}
	}

	loc, err := time.LoadLocation(zoneName)
	if err != nil {
		return time.Time{}, fmt.Errorf("time: load location %q: %w", zoneName, err)
	}

	t, err := time.ParseInLocation("2006-01-02 15:04", l.GetDate()+" "+clock, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("time: parse date %q time %q: %w", l.GetDate(), clock, err)
	}
	return t.UTC(), nil
}

// parseHHMM parses a "HH:MM" wall-clock string into hour and minute.
func parseHHMM(s string) (hour, minute int, err error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, 0, fmt.Errorf("time: parse HH:MM %q: %w", s, err)
	}
	return t.Hour(), t.Minute(), nil
}
