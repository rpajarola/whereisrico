package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rpajarola/whereisrico/internal/geo/airport"
	"github.com/rpajarola/whereisrico/internal/geo/tz"
	"github.com/rpajarola/whereisrico/internal/ingest/gpx"
	"github.com/rpajarola/whereisrico/internal/ingest/trip"
	"github.com/rpajarola/whereisrico/internal/storage"
)

func runIngest(args []string) error {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	dbPath := fs.String("db", "data/whereisrico.db", "path to the sqlite database")
	dir := fs.String("dir", "data", "directory to scan (recursively) for .gpx and .textproto files")
	force := fs.Bool("force", false, "re-ingest every matching file, even if unchanged since the last run")
	if err := fs.Parse(args); err != nil {
		return err
	}

	db, err := storage.Open(*dbPath)
	if err != nil {
		return fmt.Errorf("ingest: open db: %w", err)
	}
	defer db.Close()

	tzLookup, err := tz.New()
	if err != nil {
		return fmt.Errorf("ingest: init timezone lookup: %w", err)
	}
	airportLookup, err := airport.New()
	if err != nil {
		return fmt.Errorf("ingest: init airport lookup: %w", err)
	}
	loader := &trip.Loader{Airports: airportLookup, TZ: tzLookup}

	ctx := context.Background()
	var gpxFiles, tripFiles, skipped, gpxCoords, tripRows int

	err = filepath.WalkDir(*dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".gpx" && ext != ".textproto" {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}

		if !*force {
			needs, err := db.NeedsIngest(ctx, path, info.Size(), info.ModTime())
			if err != nil {
				return fmt.Errorf("check %s: %w", path, err)
			}
			if !needs {
				skipped++
				return nil
			}
		}

		switch ext {
		case ".gpx":
			n, err := ingestGPX(ctx, db, path)
			if err != nil {
				return err
			}
			gpxFiles++
			gpxCoords += n

		case ".textproto":
			n, err := ingestTrip(ctx, db, loader, path)
			if err != nil {
				return err
			}
			tripFiles++
			tripRows += n
		}

		if err := db.MarkIngested(ctx, path, info.Size(), info.ModTime()); err != nil {
			return fmt.Errorf("mark %s ingested: %w", path, err)
		}
		fmt.Printf("ingested %s\n", path)
		return nil
	})
	if err != nil {
		return fmt.Errorf("ingest: %w", err)
	}

	fmt.Printf("done: %d gpx files (%d coords), %d trip logs (%d rows), %d files unchanged/skipped\n",
		gpxFiles, gpxCoords, tripFiles, tripRows, skipped)
	return nil
}

func ingestGPX(ctx context.Context, db *storage.DB, path string) (int, error) {
	pts, err := gpx.ParseFile(path)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", path, err)
	}
	coords := make([]storage.Coord, len(pts))
	for i, p := range pts {
		coords[i] = storage.Coord{
			Timestamp: p.Time,
			Message:   p.Message,
			Longitude: p.Longitude,
			Latitude:  p.Latitude,
			Source:    path,
		}
	}
	n, err := db.InsertCoords(ctx, coords)
	if err != nil {
		return 0, fmt.Errorf("insert coords from %s: %w", path, err)
	}
	return n, nil
}

func ingestTrip(ctx context.Context, db *storage.DB, loader *trip.Loader, path string) (int, error) {
	result, err := loader.LoadFile(path)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", path, err)
	}
	for _, t := range result.Trips {
		if _, err := db.InsertTrip(ctx, t); err != nil {
			return 0, fmt.Errorf("insert trip from %s: %w", path, err)
		}
	}
	n, err := db.InsertCoords(ctx, result.Coords)
	if err != nil {
		return 0, fmt.Errorf("insert coords from %s: %w", path, err)
	}
	return n + len(result.Trips), nil
}
