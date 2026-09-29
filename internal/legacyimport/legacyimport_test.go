package legacyimport

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rpajarola/whereisrico/internal/storage"
)

func unixT(sec int64) time.Time { return time.Unix(sec, 0).UTC() }

// buildFixtureDB creates a SQLite file at path using the *old* Python
// system's schema (see old/whereisrico_lib.py InitDB / the real
// old/whereisrico.db .schema), seeded with a few rows -- including a
// manual.csx-sourced coord and trip, to exercise --exclude-source-glob.
func buildFixtureDB(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open fixture db: %v", err)
	}
	defer db.Close()

	stmts := []string{
		`CREATE TABLE coords (timestamp int, message text, longitude float, latitude float, source text)`,
		`CREATE TABLE trips (timestamp int, name text)`,
		`INSERT INTO coords VALUES (100, '', 8.5, 47.4, '/data/bhutan2010.gpx')`,
		`INSERT INTO coords VALUES (200, 'Zurich', 8.5, 47.4, '/data/manual.csx')`,
		`INSERT INTO coords VALUES (300, '', 89.4, 27.5, '/data/bhutan2010.gpx')`,
		`INSERT INTO trips VALUES (50, 'Bhutan 2010')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("exec %q: %v", s, err)
		}
	}
}

func TestImportBasic(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.db")
	buildFixtureDB(t, oldPath)

	newDB, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer newDB.Close()

	ctx := context.Background()
	result, err := Import(ctx, oldPath, newDB, Options{})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.CoordsSeen != 3 || result.CoordsInserted != 3 {
		t.Errorf("coords: seen=%d inserted=%d, want seen=3 inserted=3", result.CoordsSeen, result.CoordsInserted)
	}
	if result.TripsSeen != 1 || result.TripsInserted != 1 {
		t.Errorf("trips: seen=%d inserted=%d, want seen=1 inserted=1", result.TripsSeen, result.TripsInserted)
	}

	trips, err := newDB.Trips(ctx)
	if err != nil {
		t.Fatalf("Trips: %v", err)
	}
	if len(trips) != 1 || trips[0].Name != "Bhutan 2010" {
		t.Fatalf("Trips: got %+v, want exactly one Bhutan 2010 trip", trips)
	}
}

func TestImportExcludeSourceGlob(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.db")
	buildFixtureDB(t, oldPath)

	newDB, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer newDB.Close()

	ctx := context.Background()
	result, err := Import(ctx, oldPath, newDB, Options{ExcludeSourceGlob: "%manual.csx%"})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	// 3 coords in the fixture, 1 sourced from manual.csx -> 2 remain.
	if result.CoordsSeen != 2 || result.CoordsInserted != 2 {
		t.Errorf("coords: seen=%d inserted=%d, want seen=2 inserted=2 (manual.csx row excluded)", result.CoordsSeen, result.CoordsInserted)
	}
	// All old trips came from manual.csx -> none imported.
	if result.TripsSeen != 0 || result.TripsInserted != 0 {
		t.Errorf("trips: seen=%d inserted=%d, want 0/0 (all old trips are manual.csx-sourced)", result.TripsSeen, result.TripsInserted)
	}

	coords, err := newDB.CoordsBetween(ctx, unixT(0), unixT(1000))
	if err != nil {
		t.Fatalf("CoordsBetween: %v", err)
	}
	for _, c := range coords {
		if c.Message == "Zurich" {
			t.Errorf("found manual.csx-sourced coord %+v, want it excluded", c)
		}
	}
}

func TestImportIdempotent(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.db")
	buildFixtureDB(t, oldPath)

	newDB, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer newDB.Close()

	ctx := context.Background()
	if _, err := Import(ctx, oldPath, newDB, Options{}); err != nil {
		t.Fatalf("first Import: %v", err)
	}
	result, err := Import(ctx, oldPath, newDB, Options{})
	if err != nil {
		t.Fatalf("second Import: %v", err)
	}
	if result.CoordsInserted != 0 || result.TripsInserted != 0 {
		t.Errorf("second import: got %d coords / %d trips inserted, want 0/0 (all duplicates)", result.CoordsInserted, result.TripsInserted)
	}

	coords, err := newDB.CoordsBetween(ctx, unixT(0), unixT(1000))
	if err != nil {
		t.Fatalf("CoordsBetween: %v", err)
	}
	if len(coords) != 3 {
		t.Fatalf("after re-import: got %d coords, want still 3 (no duplicates)", len(coords))
	}
}
