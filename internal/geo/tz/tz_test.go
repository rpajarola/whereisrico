package tz

import "testing"

func TestZoneNameKnownPoints(t *testing.T) {
	lookup, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cases := []struct {
		name     string
		lat, lon float64
		wantZone string
	}{
		{"Zurich", 47.3769, 8.5417, "Europe/Zurich"},
		{"San Francisco", 37.7749, -122.4194, "America/Los_Angeles"},
		{"Sydney", -33.8688, 151.2093, "Australia/Sydney"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := lookup.ZoneName(c.lat, c.lon)
			if err != nil {
				t.Fatalf("ZoneName(%v, %v): %v", c.lat, c.lon, err)
			}
			if got != c.wantZone {
				t.Errorf("ZoneName(%v, %v) = %q, want %q", c.lat, c.lon, got, c.wantZone)
			}
		})
	}
}
