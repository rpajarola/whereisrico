package gpx

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	ingestgpx "github.com/rpajarola/whereisrico/internal/ingest/gpx"
	"github.com/rpajarola/whereisrico/internal/storage"
)

func ts(unix int64) time.Time { return time.Unix(unix, 0).UTC() }

func TestWriteWaypointsAndTrack(t *testing.T) {
	coords := []storage.Coord{
		// Deliberately out of chronological order, to exercise sorting.
		{Timestamp: ts(200), Longitude: 2, Latitude: 2, Source: "a"},
		{Timestamp: ts(100), Longitude: 1, Latitude: 1, Source: "a", Message: "Start"},
		{Timestamp: ts(300), Longitude: 3, Latitude: 3, Source: "a", Message: "End"},
	}

	var buf bytes.Buffer
	if err := Write(&buf, "Test Trip", coords); err != nil {
		t.Fatalf("Write: %v", err)
	}

	points, err := ingestgpx.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("re-parsing written GPX: %v\noutput was:\n%s", err, buf.String())
	}

	// 2 waypoints (Start, End) + 3 track points = 5 parsed points.
	if len(points) != 5 {
		t.Fatalf("got %d parsed points, want 5 (2 waypoints + 3 track points); output:\n%s", len(points), buf.String())
	}

	var waypoints, trackPoints int
	for _, p := range points {
		if p.Message != "" {
			waypoints++
		} else {
			trackPoints++
		}
	}
	if waypoints != 2 {
		t.Errorf("got %d waypoints, want 2", waypoints)
	}
	if trackPoints != 3 {
		t.Errorf("got %d track points, want 3", trackPoints)
	}

	// Track points must come out in chronological order regardless of
	// input order.
	var trackTimes []time.Time
	for _, p := range points {
		if p.Message == "" {
			trackTimes = append(trackTimes, p.Time)
		}
	}
	for i := 1; i < len(trackTimes); i++ {
		if trackTimes[i].Before(trackTimes[i-1]) {
			t.Errorf("track points not in chronological order: %v", trackTimes)
		}
	}
}

func TestWriteEmptyCoords(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, "Empty Trip", nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	points, err := ingestgpx.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("re-parsing written GPX: %v\noutput was:\n%s", err, buf.String())
	}
	if len(points) != 0 {
		t.Errorf("got %d points from empty input, want 0", len(points))
	}
}

func TestWriteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trip.gpx")

	coords := []storage.Coord{
		{Timestamp: ts(100), Longitude: 1, Latitude: 1, Source: "a", Message: "Only point"},
	}
	if err := WriteFile(path, "Trip", coords); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	points, err := ingestgpx.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(points) != 2 { // 1 waypoint + 1 track point
		t.Fatalf("got %d points, want 2", len(points))
	}
}

// TestRoundTripDedup verifies the documented round-trip property: after
// exporting coords with a mix of messaged/unmessaged points and
// re-importing them into a fresh database, InsertCoords deduplicates the
// track-point copy of each messaged coordinate (same timestamp/lon/lat),
// leaving exactly one row per original coordinate with the message
// preserved.
func TestRoundTripDedup(t *testing.T) {
	original := []storage.Coord{
		{Timestamp: ts(100), Longitude: 1, Latitude: 1, Source: "orig", Message: "Waypoint A"},
		{Timestamp: ts(200), Longitude: 2, Latitude: 2, Source: "orig"},
	}

	var buf bytes.Buffer
	if err := Write(&buf, "Round Trip", original); err != nil {
		t.Fatalf("Write: %v", err)
	}

	points, err := ingestgpx.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("re-parsing written GPX: %v", err)
	}

	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer db.Close()

	var reimported []storage.Coord
	for _, p := range points {
		reimported = append(reimported, storage.Coord{
			Timestamp: p.Time, Message: p.Message, Longitude: p.Longitude, Latitude: p.Latitude, Source: "reimport",
		})
	}
	n, err := db.InsertCoords(t.Context(), reimported)
	if err != nil {
		t.Fatalf("InsertCoords: %v", err)
	}
	if n != len(original) {
		t.Fatalf("inserted %d rows, want %d (one per original coordinate, track-point duplicates deduplicated)", n, len(original))
	}

	got, err := db.CoordsWithMessageBetween(t.Context(), ts(0), ts(1000))
	if err != nil {
		t.Fatalf("CoordsWithMessageBetween: %v", err)
	}
	if len(got) != 1 || got[0].Message != "Waypoint A" {
		t.Fatalf("got %+v, want exactly one coord with message %q", got, "Waypoint A")
	}
}
