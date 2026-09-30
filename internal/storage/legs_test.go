package storage

import (
	"context"
	"testing"
	"time"
)

func ts(unix int64) time.Time { return time.Unix(unix, 0).UTC() }

func TestLegsNoCoords(t *testing.T) {
	db := openTestDB(t)
	legs, err := db.Legs(context.Background(), time.Unix(10000, 0))
	if err != nil {
		t.Fatalf("Legs: %v", err)
	}
	if legs != nil {
		t.Fatalf("Legs on empty db: got %v, want nil", legs)
	}
}

func TestLegsLeadingUnnamedLeg(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, err := db.InsertCoords(ctx, []Coord{
		{Timestamp: ts(100), Longitude: 1, Latitude: 1, Source: "a"},
		{Timestamp: ts(200), Longitude: 2, Latitude: 2, Source: "a"},
	}); err != nil {
		t.Fatalf("InsertCoords: %v", err)
	}
	if _, err := db.InsertTrip(ctx, Trip{Timestamp: ts(500), Name: "Named Trip", Source: "t"}); err != nil {
		t.Fatalf("InsertTrip: %v", err)
	}
	if _, err := db.InsertCoords(ctx, []Coord{
		{Timestamp: ts(600), Longitude: 3, Latitude: 3, Source: "a"},
	}); err != nil {
		t.Fatalf("InsertCoords: %v", err)
	}

	legs, err := db.Legs(ctx, ts(10000))
	if err != nil {
		t.Fatalf("Legs: %v", err)
	}
	if len(legs) != 2 {
		t.Fatalf("got %d legs, want 2 (synthesized leading + named)", len(legs))
	}
	if legs[0].Name != "" || !legs[0].Start.Equal(ts(100)) || !legs[0].End.Equal(ts(500)) {
		t.Errorf("leg 0 = %+v, want unnamed [100,500)", legs[0])
	}
	if legs[1].Name != "Named Trip" || !legs[1].Start.Equal(ts(500)) || !legs[1].End.Equal(ts(601)) {
		t.Errorf("leg 1 = %+v, want \"Named Trip\" [500,601)", legs[1])
	}
}

func TestLegsNoLeadingLegWhenTripCoversEarliestCoord(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if _, err := db.InsertTrip(ctx, Trip{Timestamp: ts(100), Name: "Only Trip", Source: "t"}); err != nil {
		t.Fatalf("InsertTrip: %v", err)
	}
	if _, err := db.InsertCoords(ctx, []Coord{
		{Timestamp: ts(100), Longitude: 1, Latitude: 1, Source: "a"},
		{Timestamp: ts(200), Longitude: 2, Latitude: 2, Source: "a"},
	}); err != nil {
		t.Fatalf("InsertCoords: %v", err)
	}

	legs, err := db.Legs(ctx, ts(10000))
	if err != nil {
		t.Fatalf("Legs: %v", err)
	}
	if len(legs) != 1 {
		t.Fatalf("got %d legs, want 1 (no leading leg needed)", len(legs))
	}
	if legs[0].Name != "Only Trip" {
		t.Errorf("leg 0 name = %q, want %q", legs[0].Name, "Only Trip")
	}
}

func TestLegsCoversAllCoordsExactly(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	for i, name := range []string{"Trip A", "Trip B", "Trip C"} {
		start := ts(int64(1000 * (i + 1)))
		if _, err := db.InsertTrip(ctx, Trip{Timestamp: start, Name: name, Source: "t"}); err != nil {
			t.Fatalf("InsertTrip %q: %v", name, err)
		}
		if _, err := db.InsertCoords(ctx, []Coord{
			{Timestamp: start.Add(time.Second), Longitude: float64(i), Latitude: float64(i), Source: "t"},
		}); err != nil {
			t.Fatalf("InsertCoords %q: %v", name, err)
		}
	}

	legs, err := db.Legs(ctx, ts(100000))
	if err != nil {
		t.Fatalf("Legs: %v", err)
	}
	if len(legs) != 3 {
		t.Fatalf("got %d legs, want 3", len(legs))
	}
	for i, lg := range legs {
		coords, err := db.CoordsBetween(ctx, lg.Start, lg.End)
		if err != nil {
			t.Fatalf("CoordsBetween leg %d: %v", i, err)
		}
		if len(coords) != 1 {
			t.Errorf("leg %d (%q): got %d coords, want exactly 1", i, lg.Name, len(coords))
		}
	}
}
