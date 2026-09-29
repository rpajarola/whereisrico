package geojson

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rpajarola/whereisrico/internal/storage"
)

func openTestDB(t *testing.T) *storage.DB {
	t.Helper()
	db, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func ts(unix int64) time.Time { return time.Unix(unix, 0).UTC() }

func countByKind(fc *FeatureCollection, kind string) int {
	n := 0
	for _, f := range fc.Features {
		if f.Properties.Kind == kind {
			n++
		}
	}
	return n
}

func TestBuildEmptyDB(t *testing.T) {
	db := openTestDB(t)
	fc, err := Build(context.Background(), db, Options{Now: ts(10000)})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(fc.Features) != 0 {
		t.Fatalf("empty db: got %d features, want 0", len(fc.Features))
	}
	if fc.Meta.MinTimestamp != 0 || fc.Meta.MaxTimestamp != 0 {
		t.Errorf("empty db: meta = %+v, want zero range", fc.Meta)
	}
}

func TestBuildLeadingUnnamedLeg(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// Coords before the first named trip -- old GetTrips() synthesizes a
	// leading unnamed leg to cover them.
	if _, err := db.InsertCoords(ctx, []storage.Coord{
		{Timestamp: ts(100), Longitude: 1, Latitude: 1, Source: "gpx"},
		{Timestamp: ts(200), Longitude: 2, Latitude: 2, Source: "gpx"},
	}); err != nil {
		t.Fatalf("InsertCoords: %v", err)
	}
	if _, err := db.InsertTrip(ctx, storage.Trip{Timestamp: ts(500), Name: "Named Trip", Source: "textproto"}); err != nil {
		t.Fatalf("InsertTrip: %v", err)
	}
	if _, err := db.InsertCoords(ctx, []storage.Coord{
		{Timestamp: ts(600), Longitude: 3, Latitude: 3, Source: "gpx"},
	}); err != nil {
		t.Fatalf("InsertCoords: %v", err)
	}

	fc, err := Build(ctx, db, Options{Now: ts(10000)})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	trips := 0
	for _, f := range fc.Features {
		if f.Properties.Kind != "trip" {
			continue
		}
		trips++
		if f.Properties.TripIndex == 0 && f.Properties.TripName != "" {
			t.Errorf("leg 0 (synthesized leading leg): name = %q, want empty", f.Properties.TripName)
		}
		if f.Properties.TripIndex == 1 && f.Properties.TripName != "Named Trip" {
			t.Errorf("leg 1: name = %q, want %q", f.Properties.TripName, "Named Trip")
		}
	}
	if trips != 2 {
		t.Fatalf("got %d trip-leg features, want 2 (synthesized leading + named)", trips)
	}
}

func TestBuildColorCyclingAndLatestTripRed(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// 7 trips (> numColors=5) to exercise index wraparound; each with one
	// coordinate so every leg produces a "trip" LineString feature.
	for i := 0; i < 7; i++ {
		start := ts(int64(1000 * (i + 1)))
		name := fmt.Sprintf("leg %d", i)
		if _, err := db.InsertTrip(ctx, storage.Trip{Timestamp: start, Name: name, Source: "t"}); err != nil {
			t.Fatalf("InsertTrip %d: %v", i, err)
		}
		if _, err := db.InsertCoords(ctx, []storage.Coord{
			{Timestamp: start.Add(time.Second), Longitude: float64(i), Latitude: float64(i), Source: "t"},
		}); err != nil {
			t.Fatalf("InsertCoords %d: %v", i, err)
		}
	}

	fc, err := Build(ctx, db, Options{Now: ts(100000)})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	gotByIndex := map[int]Properties{}
	for _, f := range fc.Features {
		if f.Properties.Kind == "trip" {
			gotByIndex[f.Properties.TripIndex] = f.Properties
		}
	}
	if len(gotByIndex) != 7 {
		t.Fatalf("got %d trip legs, want 7", len(gotByIndex))
	}
	for i := 0; i < 7; i++ {
		p, ok := gotByIndex[i]
		if !ok {
			t.Fatalf("missing trip leg %d", i)
		}
		wantColor := i % numColors
		if p.ColorIndex != wantColor {
			t.Errorf("leg %d: color_index = %d, want %d", i, p.ColorIndex, wantColor)
		}
		wantLatest := i == 6
		if p.IsLatestTrip != wantLatest {
			t.Errorf("leg %d: is_latest_trip = %v, want %v", i, p.IsLatestTrip, wantLatest)
		}
	}
}

func TestBuildIsHereExactlyOnePoint(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, err := db.InsertCoords(ctx, []storage.Coord{
		{Timestamp: ts(100), Longitude: 1, Latitude: 1, Source: "t", Message: "start"},
		{Timestamp: ts(200), Longitude: 2, Latitude: 2, Source: "t", Message: "middle"},
		{Timestamp: ts(300), Longitude: 3, Latitude: 3, Source: "t", Message: "latest, has message"},
	}); err != nil {
		t.Fatalf("InsertCoords: %v", err)
	}

	fc, err := Build(ctx, db, Options{Now: ts(100000)})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	hereCount := 0
	var hereMsg string
	for _, f := range fc.Features {
		if f.Properties.IsHere {
			hereCount++
			hereMsg = f.Properties.Message
		}
	}
	if hereCount != 1 {
		t.Fatalf("got %d is_here features, want exactly 1", hereCount)
	}
	if hereMsg != "latest, has message" {
		t.Errorf("is_here feature message = %q, want the most recent coord's own message", hereMsg)
	}
}

func TestBuildSynthesizesHereMarkerWhenLatestHasNoMessage(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, err := db.InsertCoords(ctx, []storage.Coord{
		{Timestamp: ts(100), Longitude: 1, Latitude: 1, Source: "t", Message: "start"},
		{Timestamp: ts(300), Longitude: 3, Latitude: 3, Source: "t"}, // most recent, no message
	}); err != nil {
		t.Fatalf("InsertCoords: %v", err)
	}

	fc, err := Build(ctx, db, Options{Now: ts(100000)})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	hereCount := 0
	var here Properties
	for _, f := range fc.Features {
		if f.Properties.IsHere {
			hereCount++
			here = f.Properties
		}
	}
	if hereCount != 1 {
		t.Fatalf("got %d is_here features, want exactly 1", hereCount)
	}
	if here.Message != "I am here" {
		t.Errorf("synthesized here marker message = %q, want %q", here.Message, "I am here")
	}
	if here.Timestamp != 300 {
		t.Errorf("synthesized here marker timestamp = %d, want 300", here.Timestamp)
	}
}

func TestBuildMetaTimeRange(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, err := db.InsertCoords(ctx, []storage.Coord{
		{Timestamp: ts(100), Longitude: 1, Latitude: 1, Source: "t"},
		{Timestamp: ts(500), Longitude: 2, Latitude: 2, Source: "t"},
		{Timestamp: ts(99999), Longitude: 3, Latitude: 3, Source: "t"}, // excluded by Now
	}); err != nil {
		t.Fatalf("InsertCoords: %v", err)
	}

	fc, err := Build(ctx, db, Options{Now: ts(1000)})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if fc.Meta.MinTimestamp != 100 || fc.Meta.MaxTimestamp != 500 {
		t.Errorf("meta = %+v, want min=100 max=500", fc.Meta)
	}
}
