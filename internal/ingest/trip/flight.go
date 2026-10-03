package trip

import (
	"fmt"
	"regexp"
	"time"

	tripv1 "github.com/rpajarola/whereisrico/internal/gen/trip/v1"
	"github.com/rpajarola/whereisrico/internal/geo/airport"
	"github.com/rpajarola/whereisrico/internal/geo/tz"
	"github.com/rpajarola/whereisrico/internal/storage"
)

// expandFlight resolves a Flight entry into an origin coordinate (departure
// airport at departure time) and a destination coordinate (arrival airport
// at its local arrival time), mirroring the old computeFlight() but fixing
// two bugs in it:
//
//  1. The old code combined the arrival time-of-day with the *host
//     machine's local calendar date* (via datetime.fromtimestamp, which is
//     timezone-dependent on whatever machine runs the script) instead of
//     the origin airport's own local date. This version derives the date
//     from the origin airport's own resolved timezone, which is
//     deterministic regardless of where the ingestion process runs.
//  2. When the naive arrival instant lands before the departure instant
//     (an overnight flight crossing into the next calendar day), the old
//     code corrected it with `dest_timestamp *= 24 * 3600` -- multiplying a
//     Unix timestamp instead of adding a day. This version adds 24 hours,
//     repeatedly if needed.
func expandFlight(source string, f *tripv1.Flight, airports airport.Lookup, tzs tz.Lookup) (origin, dest storage.Coord, err error) {
	if f.GetOriginAirport() == "" || f.GetDestinationAirport() == "" {
		return storage.Coord{}, storage.Coord{}, fmt.Errorf("flight: origin and destination airport codes are required")
	}
	if f.GetArrivalLocalTime() == "" {
		return storage.Coord{}, storage.Coord{}, fmt.Errorf("flight: arrival_local_time is required")
	}

	originAirport, ok := airports.ByIATA(f.GetOriginAirport())
	if !ok {
		return storage.Coord{}, storage.Coord{}, fmt.Errorf("flight: unknown origin airport %q", f.GetOriginAirport())
	}
	destAirport, ok := airports.ByIATA(f.GetDestinationAirport())
	if !ok {
		return storage.Coord{}, storage.Coord{}, fmt.Errorf("flight: unknown destination airport %q", f.GetDestinationAirport())
	}

	originTime, err := resolveTimeSpec(f.GetDepartureTime(), true, originAirport.Latitude, originAirport.Longitude, tzs)
	if err != nil {
		return storage.Coord{}, storage.Coord{}, fmt.Errorf("flight: resolve departure time: %w", err)
	}

	originZoneName, err := tzs.ZoneName(originAirport.Latitude, originAirport.Longitude)
	if err != nil {
		return storage.Coord{}, storage.Coord{}, fmt.Errorf("flight: resolve origin timezone: %w", err)
	}
	originLoc, err := time.LoadLocation(originZoneName)
	if err != nil {
		return storage.Coord{}, storage.Coord{}, fmt.Errorf("flight: load origin location %q: %w", originZoneName, err)
	}
	originDate := originTime.In(originLoc)

	arrHour, arrMinute, err := parseHHMM(f.GetArrivalLocalTime())
	if err != nil {
		return storage.Coord{}, storage.Coord{}, fmt.Errorf("flight: parse arrival_local_time: %w", err)
	}

	destZoneName, err := tzs.ZoneName(destAirport.Latitude, destAirport.Longitude)
	if err != nil {
		return storage.Coord{}, storage.Coord{}, fmt.Errorf("flight: resolve destination timezone: %w", err)
	}
	destLoc, err := time.LoadLocation(destZoneName)
	if err != nil {
		return storage.Coord{}, storage.Coord{}, fmt.Errorf("flight: load destination location %q: %w", destZoneName, err)
	}

	destTime := time.Date(originDate.Year(), originDate.Month(), originDate.Day(),
		arrHour, arrMinute, 0, 0, destLoc).UTC()
	// Eastbound flights across the date line (e.g. SFO 22:25 -> SYD 06:05)
	// arrive two calendar days after departure, so one day may not be
	// enough.
	for destTime.Before(originTime) {
		destTime = destTime.Add(24 * time.Hour)
	}

	origin = storage.Coord{
		Timestamp: originTime,
		Message:   fmt.Sprintf(flightOriginFormat, f.GetOriginAirport(), f.GetAirline(), f.GetFlightNumber(), f.GetDestinationAirport()),
		Longitude: originAirport.Longitude,
		Latitude:  originAirport.Latitude,
		Source:    source,
	}
	dest = storage.Coord{
		Timestamp: destTime,
		Message:   f.GetDestinationAirport(),
		Longitude: destAirport.Longitude,
		Latitude:  destAirport.Latitude,
		Source:    source,
	}
	return origin, dest, nil
}

// flightOriginFormat is the message expandFlight puts on a flight's origin
// coordinate; flightOriginRE matches it. The destination coordinate's
// message is just the destination airport code.
const flightOriginFormat = "%s (%s %s -> %s)"

var flightOriginRE = regexp.MustCompile(`^(\S+) \(.* -> (\S+)\)$`)

// ParseFlightOrigin reports whether msg is a flight origin coordinate's
// message as produced by expandFlight, and if so returns the origin and
// destination airport codes. The matching destination coordinate is the
// next one whose message equals dest.
func ParseFlightOrigin(msg string) (origin, dest string, ok bool) {
	m := flightOriginRE.FindStringSubmatch(msg)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}
