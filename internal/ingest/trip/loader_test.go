package trip

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/prototext"

	tripv1 "github.com/rpajarola/whereisrico/internal/gen/trip/v1"
)

func mustParse(t *testing.T, text string) *tripv1.TripLog {
	t.Helper()
	var log tripv1.TripLog
	if err := prototext.Unmarshal([]byte(text), &log); err != nil {
		t.Fatalf("prototext.Unmarshal: %v", err)
	}
	return &log
}

func TestLoadTripStartAndWaypoint(t *testing.T) {
	const text = `
entries {
  trip_start {
    time { local { date: "2024-06-25" time: "22:25" zone: "Europe/Zurich" } }
    name: "Australia 2024"
  }
}
entries {
  waypoint {
    time { local { date: "2024-06-26" time: "10:00" } }
    latitude: -33.8688
    longitude: 151.2093
    message: "Sydney Opera House"
  }
}
`
	l := &Loader{
		Airports: fakeAirports{},
		TZ:       fakeTZ{zone: "Australia/Sydney"},
	}
	result, err := l.Load("test.textproto", mustParse(t, text))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(result.Trips) != 1 {
		t.Fatalf("got %d trips, want 1", len(result.Trips))
	}
	if result.Trips[0].Name != "Australia 2024" {
		t.Errorf("trip name = %q, want %q", result.Trips[0].Name, "Australia 2024")
	}
	wantTripTime := time.Date(2024, 6, 25, 20, 25, 0, 0, time.UTC) // 22:25 CEST = 20:25 UTC
	if !result.Trips[0].Timestamp.Equal(wantTripTime) {
		t.Errorf("trip timestamp = %v, want %v", result.Trips[0].Timestamp, wantTripTime)
	}

	if len(result.Coords) != 1 {
		t.Fatalf("got %d coords, want 1", len(result.Coords))
	}
	if result.Coords[0].Message != "Sydney Opera House" {
		t.Errorf("waypoint message = %q, want %q", result.Coords[0].Message, "Sydney Opera House")
	}
	wantWptTime := time.Date(2024, 6, 26, 0, 0, 0, 0, time.UTC) // 10:00 AEST (+10) = 00:00 UTC
	if !result.Coords[0].Timestamp.Equal(wantWptTime) {
		t.Errorf("waypoint timestamp = %v, want %v (zone resolved from coordinates)", result.Coords[0].Timestamp, wantWptTime)
	}
	if result.Coords[0].Source != "test.textproto" {
		t.Errorf("waypoint source = %q, want %q", result.Coords[0].Source, "test.textproto")
	}
}

func TestLoadAbsoluteTimestamp(t *testing.T) {
	const text = `
entries {
  waypoint {
    time { absolute { seconds: 1000000000 } }
    latitude: 1
    longitude: 2
    message: "absolute time test"
  }
}
`
	l := &Loader{Airports: fakeAirports{}, TZ: fakeTZ{zone: "UTC"}}
	result, err := l.Load("src", mustParse(t, text))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := time.Unix(1000000000, 0).UTC()
	if !result.Coords[0].Timestamp.Equal(want) {
		t.Errorf("timestamp = %v, want %v", result.Coords[0].Timestamp, want)
	}
}

func TestLoadValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{
			name: "trip_start missing name",
			text: `entries { trip_start { time { local { date: "2024-01-01" } } } }`,
		},
		{
			name: "local date time missing date",
			text: `entries { trip_start { time { local { time: "10:00" } } name: "x" } }`,
		},
		{
			name: "time spec with neither absolute nor local",
			text: `entries { trip_start { time {} name: "x" } }`,
		},
		{
			name: "waypoint with no zone and time spec present but unresolvable via TZ",
			text: `entries { waypoint { time { local { date: "2024-01-01" } } latitude: 0 longitude: 0 } }`,
		},
		{
			name: "entry with none of trip_start/waypoint/flight set",
			text: `entries {}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := &Loader{Airports: fakeAirports{}, TZ: erroringTZ{}}
			if _, err := l.Load("src", mustParse(t, c.text)); err == nil {
				t.Fatalf("Load(%q): want error, got nil", c.text)
			}
		})
	}
}

// erroringTZ always fails coordinate-based zone resolution, so validation
// tests that rely on "no zone, coords present" still exercise a real error
// path rather than accidentally succeeding.
type erroringTZ struct{}

func (erroringTZ) ZoneName(lat, lon float64) (string, error) {
	return "", fmt.Errorf("trip: zone not configured for test")
}

func TestLoadRejectsUnknownField(t *testing.T) {
	// A typo'd field name is a parse-time error, unlike the old CSX format
	// where a typo like "FILGHT" was silently dropped with no error at all.
	const text = `entries { trip_start { time { local { dat: "2024-01-01" } } name: "x" } }`
	var log tripv1.TripLog
	err := prototext.Unmarshal([]byte(text), &log)
	if err == nil {
		t.Fatal("prototext.Unmarshal with unknown field \"dat\": want error, got nil")
	}
	if !strings.Contains(err.Error(), "dat") {
		t.Errorf("error %v does not mention the unknown field", err)
	}
}

func TestLoadFileRealFixture(t *testing.T) {
	l := &Loader{
		Airports: fakeAirports{},
		TZ:       fakeTZ{zone: "Europe/Zurich"},
	}
	result, err := l.LoadFile("testdata/simple_trip.textproto")
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(result.Trips) != 1 || len(result.Coords) != 2 {
		t.Fatalf("got %d trips, %d coords; want 1 trip, 2 coords", len(result.Trips), len(result.Coords))
	}
}
