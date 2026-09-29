package trip

import (
	"testing"
	"time"

	tripv1 "github.com/rpajarola/whereisrico/internal/gen/trip/v1"
	"github.com/rpajarola/whereisrico/internal/geo/airport"
)

func testAirports() fakeAirports {
	return fakeAirports{byCode: map[string]airport.Airport{
		"ZRH": {IATA: "ZRH", Name: "Zurich Airport", Latitude: 47.45, Longitude: 8.56},
		"SFO": {IATA: "SFO", Name: "San Francisco Intl", Latitude: 37.62, Longitude: -122.38},
		"SYD": {IATA: "SYD", Name: "Sydney Airport", Latitude: -33.94, Longitude: 151.18},
	}}
}

func testZones() fakeZonedTZ {
	return fakeZonedTZ{byCoord: map[[2]float64]string{
		{47.45, 8.56}:    "Europe/Zurich",
		{37.62, -122.38}: "America/Los_Angeles",
		{-33.94, 151.18}: "Australia/Sydney",
	}}
}

func TestExpandFlightSameDay(t *testing.T) {
	// ZRH -> SFO same-day arrival: departs 13:10 CEST, arrives 16:00 local
	// (America/Los_Angeles), which is well after departure in UTC terms.
	f := &tripv1.Flight{
		DepartureTime: &tripv1.TimeSpec{Spec: &tripv1.TimeSpec_Local{Local: &tripv1.LocalDateTime{
			Date: "2025-06-10", Time: "13:10", Zone: "Europe/Zurich",
		}}},
		Airline:            "LX",
		FlightNumber:       "38",
		OriginAirport:      "ZRH",
		DestinationAirport: "SFO",
		ArrivalLocalTime:   "16:00",
	}

	origin, dest, err := expandFlight("test", f, testAirports(), testZones())
	if err != nil {
		t.Fatalf("expandFlight: %v", err)
	}

	wantOrigin := time.Date(2025, 6, 10, 11, 10, 0, 0, time.UTC) // 13:10 CEST (+2)
	if !origin.Timestamp.Equal(wantOrigin) {
		t.Errorf("origin timestamp = %v, want %v", origin.Timestamp, wantOrigin)
	}
	if origin.Message != "ZRH (LX 38 -> SFO)" {
		t.Errorf("origin message = %q, want %q", origin.Message, "ZRH (LX 38 -> SFO)")
	}

	// 16:00 PDT (-7) on the origin's local calendar date (2025-06-10) = 23:00 UTC.
	wantDest := time.Date(2025, 6, 10, 23, 0, 0, 0, time.UTC)
	if !dest.Timestamp.Equal(wantDest) {
		t.Errorf("dest timestamp = %v, want %v", dest.Timestamp, wantDest)
	}
	if dest.Timestamp.Before(origin.Timestamp) {
		t.Errorf("dest timestamp %v is before origin timestamp %v", dest.Timestamp, origin.Timestamp)
	}
	if dest.Message != "SFO" {
		t.Errorf("dest message = %q, want %q", dest.Message, "SFO")
	}
}

// TestExpandFlightOvernightAddsOneDay is a regression test for a bug in the
// old Python computeFlight(): when the naive destination instant landed
// before the origin instant, it corrected this with
// `dest_timestamp *= 24 * 3600` (multiplying a Unix timestamp) instead of
// adding a day. This asserts the fixed +24h behavior.
func TestExpandFlightOvernightAddsOneDay(t *testing.T) {
	// SYD -> ZRH: departs late at night Sydney time, arrives early morning
	// Zurich time -- naively combining the origin's calendar date with the
	// destination's arrival time-of-day lands *before* departure in UTC, so
	// the loader must roll the destination forward by one day.
	f := &tripv1.Flight{
		DepartureTime: &tripv1.TimeSpec{Spec: &tripv1.TimeSpec_Local{Local: &tripv1.LocalDateTime{
			Date: "2025-06-10", Time: "23:30", Zone: "Australia/Sydney",
		}}},
		Airline:            "QF",
		FlightNumber:       "1",
		OriginAirport:      "SYD",
		DestinationAirport: "ZRH",
		ArrivalLocalTime:   "06:00",
	}

	origin, dest, err := expandFlight("test", f, testAirports(), testZones())
	if err != nil {
		t.Fatalf("expandFlight: %v", err)
	}

	wantOrigin := time.Date(2025, 6, 10, 13, 30, 0, 0, time.UTC) // 23:30 AEST (+10)
	if !origin.Timestamp.Equal(wantOrigin) {
		t.Errorf("origin timestamp = %v, want %v", origin.Timestamp, wantOrigin)
	}

	// Naive: 06:00 CEST (+2) on 2025-06-10 = 04:00 UTC, which is BEFORE the
	// 13:30 UTC departure -- must be rolled forward one day to 2025-06-11.
	wantDest := time.Date(2025, 6, 11, 4, 0, 0, 0, time.UTC)
	if !dest.Timestamp.Equal(wantDest) {
		t.Errorf("dest timestamp = %v, want %v (naive time rolled forward by exactly 24h)", dest.Timestamp, wantDest)
	}
	if dest.Timestamp.Before(origin.Timestamp) {
		t.Errorf("dest timestamp %v is before origin timestamp %v after overnight correction", dest.Timestamp, origin.Timestamp)
	}

	// The old bug (`*= 24*3600`) would have produced an absurd timestamp
	// far in the future (multiplying a ~1.75 billion-second Unix timestamp
	// by 86400) -- assert we are nowhere near that.
	if dest.Timestamp.Year() > 2030 {
		t.Errorf("dest timestamp %v looks like the old *86400 bug, not a +24h correction", dest.Timestamp)
	}
}

func TestExpandFlightUnknownAirport(t *testing.T) {
	f := &tripv1.Flight{
		DepartureTime: &tripv1.TimeSpec{Spec: &tripv1.TimeSpec_Local{Local: &tripv1.LocalDateTime{
			Date: "2025-01-01", Zone: "UTC",
		}}},
		OriginAirport:      "ZZZ",
		DestinationAirport: "SFO",
		ArrivalLocalTime:   "10:00",
	}
	if _, _, err := expandFlight("test", f, testAirports(), testZones()); err == nil {
		t.Fatal("expandFlight with unknown origin airport: want error, got nil")
	}
}
