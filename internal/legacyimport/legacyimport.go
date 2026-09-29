// Package legacyimport imports coordinates and trips from the old Python
// system's SQLite database (old/whereisrico.db) into the new schema. The
// old data is already flattened into (timestamp, message, longitude,
// latitude, source) rows by the old ingestion code, so this package has no
// need to reproduce any of that old SPOT/GPX/CSX parsing logic -- it simply
// reads the old coords/trips tables directly and re-inserts them through
// the new storage.DB, which applies the same dedup rules as any other
// ingestion path.
package legacyimport

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rpajarola/whereisrico/internal/storage"
)

// Options configures Import.
type Options struct {
	// ExcludeSourceGlob, if non-empty, is a SQL LIKE pattern; rows from the
	// old database whose source column matches it are skipped. Used to
	// exclude a legacy source (e.g. "%manual.csx%") once its data has been
	// properly re-authored as .textproto and ingested through the normal
	// path, so the legacy import doesn't reintroduce the old, uncorrected
	// version of that data alongside it.
	ExcludeSourceGlob string
}

// Result summarizes what Import did.
type Result struct {
	CoordsSeen, CoordsInserted int
	TripsSeen, TripsInserted   int
}

// Import reads coords and trips from the old SQLite database at oldDBPath
// and inserts them into newDB.
func Import(ctx context.Context, oldDBPath string, newDB *storage.DB, opts Options) (Result, error) {
	oldDB, err := sql.Open("sqlite", "file:"+oldDBPath+"?mode=ro")
	if err != nil {
		return Result{}, fmt.Errorf("legacyimport: open old db %s: %w", oldDBPath, err)
	}
	defer oldDB.Close()

	var result Result

	coords, err := readCoords(ctx, oldDB, opts.ExcludeSourceGlob)
	if err != nil {
		return Result{}, fmt.Errorf("legacyimport: read coords: %w", err)
	}
	result.CoordsSeen = len(coords)
	inserted, err := newDB.InsertCoords(ctx, coords)
	if err != nil {
		return Result{}, fmt.Errorf("legacyimport: insert coords: %w", err)
	}
	result.CoordsInserted = inserted

	trips, err := readTrips(ctx, oldDB, opts.ExcludeSourceGlob)
	if err != nil {
		return Result{}, fmt.Errorf("legacyimport: read trips: %w", err)
	}
	result.TripsSeen = len(trips)
	for _, t := range trips {
		ok, err := newDB.InsertTrip(ctx, t)
		if err != nil {
			return Result{}, fmt.Errorf("legacyimport: insert trip %q: %w", t.Name, err)
		}
		if ok {
			result.TripsInserted++
		}
	}

	return result, nil
}

func readCoords(ctx context.Context, oldDB *sql.DB, excludeGlob string) ([]storage.Coord, error) {
	query := `SELECT timestamp, message, longitude, latitude, source FROM coords`
	args := []any{}
	if excludeGlob != "" {
		query += ` WHERE source NOT LIKE ?`
		args = append(args, excludeGlob)
	}

	rows, err := oldDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []storage.Coord
	for rows.Next() {
		var c storage.Coord
		var ts int64
		if err := rows.Scan(&ts, &c.Message, &c.Longitude, &c.Latitude, &c.Source); err != nil {
			return nil, fmt.Errorf("scan coord row: %w", err)
		}
		c.Timestamp = time.Unix(ts, 0).UTC()
		out = append(out, c)
	}
	return out, rows.Err()
}

func readTrips(ctx context.Context, oldDB *sql.DB, excludeGlob string) ([]storage.Trip, error) {
	// The old trips table has no source column tracking file provenance the
	// way coords does; manual.csx is the only source of trips in the old
	// system, so any non-empty excludeGlob means "skip all old trips" --
	// they will instead come from the converted .textproto files.
	if excludeGlob != "" {
		return nil, nil
	}

	rows, err := oldDB.QueryContext(ctx, `SELECT timestamp, name FROM trips`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []storage.Trip
	for rows.Next() {
		var t storage.Trip
		var ts int64
		if err := rows.Scan(&ts, &t.Name); err != nil {
			return nil, fmt.Errorf("scan trip row: %w", err)
		}
		t.Timestamp = time.Unix(ts, 0).UTC()
		t.Source = "legacy-import"
		out = append(out, t)
	}
	return out, rows.Err()
}
