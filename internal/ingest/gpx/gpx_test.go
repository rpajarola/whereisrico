package gpx

import (
	"strings"
	"testing"
	"time"
)

const syntheticGPX = `<?xml version="1.0" encoding="UTF-8"?>
<gpx version="1.1" xmlns="http://www.topografix.com/GPX/1/1">
  <wpt lat="27.7" lon="85.3">
    <time>2017-10-01T22:07:07Z</time>
    <name>Kathmandu</name>
  </wpt>
  <trk>
    <name>Test Track</name>
    <trkseg>
      <trkpt lat="27.670376" lon="84.422615">
        <ele>1299.84</ele>
        <time>2009-09-09T05:12:29Z</time>
      </trkpt>
      <trkpt lat="27.663703" lon="84.425426">
        <time>2009-09-09T05:14:16.500Z</time>
      </trkpt>
    </trkseg>
  </trk>
</gpx>`

func TestParseSynthetic(t *testing.T) {
	pts, err := Parse(strings.NewReader(syntheticGPX))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(pts) != 3 {
		t.Fatalf("got %d points, want 3 (1 waypoint + 2 track points)", len(pts))
	}

	wpt := pts[0]
	if wpt.Message != "Kathmandu" {
		t.Errorf("waypoint message = %q, want %q", wpt.Message, "Kathmandu")
	}
	if wpt.Latitude != 27.7 || wpt.Longitude != 85.3 {
		t.Errorf("waypoint coords = (%v, %v), want (27.7, 85.3)", wpt.Latitude, wpt.Longitude)
	}
	wantTime := time.Date(2017, 10, 1, 22, 7, 7, 0, time.UTC)
	if !wpt.Time.Equal(wantTime) {
		t.Errorf("waypoint time = %v, want %v", wpt.Time, wantTime)
	}

	for _, tp := range pts[1:] {
		if tp.Message != "" {
			t.Errorf("track point message = %q, want empty", tp.Message)
		}
	}
	if pts[2].Time.Sub(pts[1].Time) != 107500*time.Millisecond {
		t.Errorf("track point time delta = %v, want 1m47.5s", pts[2].Time.Sub(pts[1].Time))
	}
}

func TestParseMissingTime(t *testing.T) {
	const badGPX = `<gpx xmlns="http://www.topografix.com/GPX/1/1">
	  <wpt lat="1" lon="2"><name>no time</name></wpt>
	</gpx>`
	if _, err := Parse(strings.NewReader(badGPX)); err == nil {
		t.Fatal("Parse with missing <time>: want error, got nil")
	}
}

func TestParseFileRealSample(t *testing.T) {
	pts, err := ParseFile("../../../old/chitwan.gpx")
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(pts) == 0 {
		t.Fatal("ParseFile: got 0 points from real sample file")
	}
	for i := 1; i < len(pts); i++ {
		if pts[i].Time.Before(pts[i-1].Time) {
			t.Fatalf("points not in chronological order at index %d: %v before %v", i, pts[i].Time, pts[i-1].Time)
		}
	}
}
