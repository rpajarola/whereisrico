package storage

import (
	"context"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestInsertCoordDedup(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	c := Coord{Timestamp: time.Unix(1000, 0), Message: "hi", Longitude: 8.5, Latitude: 47.4, Source: "test"}
	inserted, err := db.InsertCoord(ctx, c)
	if err != nil {
		t.Fatalf("InsertCoord: %v", err)
	}
	if !inserted {
		t.Fatalf("first insert: want inserted=true")
	}

	inserted, err = db.InsertCoord(ctx, c)
	if err != nil {
		t.Fatalf("InsertCoord (dup): %v", err)
	}
	if inserted {
		t.Fatalf("duplicate insert: want inserted=false")
	}

	// Different message but same (timestamp, lon, lat) is still a dup by
	// the unique index -- matches old sqlite schema's dedup semantics.
	c2 := c
	c2.Message = "different message"
	inserted, err = db.InsertCoord(ctx, c2)
	if err != nil {
		t.Fatalf("InsertCoord (dup, diff message): %v", err)
	}
	if inserted {
		t.Fatalf("duplicate (ts,lon,lat) insert: want inserted=false")
	}
}

func TestInsertCoordsBatch(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	cs := []Coord{
		{Timestamp: time.Unix(1000, 0), Longitude: 1, Latitude: 1, Source: "a"},
		{Timestamp: time.Unix(2000, 0), Longitude: 2, Latitude: 2, Source: "a"},
		{Timestamp: time.Unix(1000, 0), Longitude: 1, Latitude: 1, Source: "a"}, // dup of first
	}
	n, err := db.InsertCoords(ctx, cs)
	if err != nil {
		t.Fatalf("InsertCoords: %v", err)
	}
	if n != 2 {
		t.Fatalf("InsertCoords: got %d inserted, want 2", n)
	}
}

func TestCoordsBetweenBoundaries(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	cs := []Coord{
		{Timestamp: time.Unix(100, 0), Longitude: 1, Latitude: 1, Source: "a"},
		{Timestamp: time.Unix(200, 0), Longitude: 2, Latitude: 2, Source: "a"},
		{Timestamp: time.Unix(300, 0), Longitude: 3, Latitude: 3, Source: "a"},
	}
	if _, err := db.InsertCoords(ctx, cs); err != nil {
		t.Fatalf("InsertCoords: %v", err)
	}

	got, err := db.CoordsBetween(ctx, time.Unix(100, 0), time.Unix(300, 0))
	if err != nil {
		t.Fatalf("CoordsBetween: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("CoordsBetween [100,300): got %d rows, want 2 (100 inclusive, 300 exclusive)", len(got))
	}
	if got[0].Timestamp.Unix() != 100 || got[1].Timestamp.Unix() != 200 {
		t.Fatalf("CoordsBetween [100,300): unexpected rows %+v", got)
	}
}

func TestCoordsWithMessageBetween(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	cs := []Coord{
		{Timestamp: time.Unix(100, 0), Longitude: 1, Latitude: 1, Source: "a", Message: "has message"},
		{Timestamp: time.Unix(200, 0), Longitude: 2, Latitude: 2, Source: "a"},
	}
	if _, err := db.InsertCoords(ctx, cs); err != nil {
		t.Fatalf("InsertCoords: %v", err)
	}

	got, err := db.CoordsWithMessageBetween(ctx, time.Unix(0, 0), time.Unix(1000, 0))
	if err != nil {
		t.Fatalf("CoordsWithMessageBetween: %v", err)
	}
	if len(got) != 1 || got[0].Message != "has message" {
		t.Fatalf("CoordsWithMessageBetween: got %+v, want exactly the messaged coord", got)
	}
}

func TestTimeRange(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, _, err := db.TimeRange(ctx, time.Unix(10000, 0)); err != ErrNoCoords {
		t.Fatalf("TimeRange on empty db: got err %v, want ErrNoCoords", err)
	}

	cs := []Coord{
		{Timestamp: time.Unix(100, 0), Longitude: 1, Latitude: 1, Source: "a"},
		{Timestamp: time.Unix(500, 0), Longitude: 2, Latitude: 2, Source: "a"},
		{Timestamp: time.Unix(9000, 0), Longitude: 3, Latitude: 3, Source: "a"}, // excluded by "before"
	}
	if _, err := db.InsertCoords(ctx, cs); err != nil {
		t.Fatalf("InsertCoords: %v", err)
	}

	min, max, err := db.TimeRange(ctx, time.Unix(1000, 0))
	if err != nil {
		t.Fatalf("TimeRange: %v", err)
	}
	if min.Unix() != 100 || max.Unix() != 500 {
		t.Fatalf("TimeRange: got (%v, %v), want (100, 500)", min.Unix(), max.Unix())
	}
}

func TestInsertTripDedup(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	tr := Trip{Timestamp: time.Unix(1000, 0), Name: "Bhutan 2010", Source: "test.textproto"}
	inserted, err := db.InsertTrip(ctx, tr)
	if err != nil {
		t.Fatalf("InsertTrip: %v", err)
	}
	if !inserted {
		t.Fatalf("first insert: want inserted=true")
	}

	inserted, err = db.InsertTrip(ctx, tr)
	if err != nil {
		t.Fatalf("InsertTrip (dup): %v", err)
	}
	if inserted {
		t.Fatalf("duplicate insert: want inserted=false")
	}

	trips, err := db.Trips(ctx)
	if err != nil {
		t.Fatalf("Trips: %v", err)
	}
	if len(trips) != 1 || trips[0].Name != "Bhutan 2010" {
		t.Fatalf("Trips: got %+v, want exactly one Bhutan 2010 trip", trips)
	}
}

func TestIngestedFiles(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	modTime := time.Unix(5000, 0)
	needs, err := db.NeedsIngest(ctx, "foo.gpx", 100, modTime)
	if err != nil {
		t.Fatalf("NeedsIngest: %v", err)
	}
	if !needs {
		t.Fatalf("unseen file: want needs=true")
	}

	if err := db.MarkIngested(ctx, "foo.gpx", 100, modTime); err != nil {
		t.Fatalf("MarkIngested: %v", err)
	}

	needs, err = db.NeedsIngest(ctx, "foo.gpx", 100, modTime)
	if err != nil {
		t.Fatalf("NeedsIngest (unchanged): %v", err)
	}
	if needs {
		t.Fatalf("unchanged file: want needs=false")
	}

	needs, err = db.NeedsIngest(ctx, "foo.gpx", 200, modTime)
	if err != nil {
		t.Fatalf("NeedsIngest (changed size): %v", err)
	}
	if !needs {
		t.Fatalf("changed-size file: want needs=true")
	}
}
