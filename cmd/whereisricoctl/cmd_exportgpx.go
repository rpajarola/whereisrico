package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	exportgpx "github.com/rpajarola/whereisrico/internal/export/gpx"
	"github.com/rpajarola/whereisrico/internal/storage"
)

func runExportGPX(args []string) error {
	fs := flag.NewFlagSet("export-gpx", flag.ExitOnError)
	dbPath := fs.String("db", "data/whereisrico.db", "path to the sqlite database")
	outDir := fs.String("out-dir", "data/gpx-export", "directory to write one .gpx file per trip into")
	tripName := fs.String("trip", "", "export only the trip with this exact name (default: every named trip)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	db, err := storage.Open(*dbPath)
	if err != nil {
		return fmt.Errorf("export-gpx: open db: %w", err)
	}
	defer db.Close()

	ctx := context.Background()
	legs, err := db.Legs(ctx, time.Now())
	if err != nil {
		return fmt.Errorf("export-gpx: %w", err)
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return fmt.Errorf("export-gpx: mkdir %s: %w", *outDir, err)
	}

	written := 0
	for _, lg := range legs {
		if lg.Name == "" {
			// The synthesized leading leg (coordinates before the first
			// named trip) has no trip to export it under.
			continue
		}
		if *tripName != "" && lg.Name != *tripName {
			continue
		}

		coords, err := db.CoordsBetween(ctx, lg.Start, lg.End)
		if err != nil {
			return fmt.Errorf("export-gpx: coords for %q: %w", lg.Name, err)
		}
		if len(coords) == 0 {
			continue
		}

		outPath := filepath.Join(*outDir, exportSlugify(lg.Name)+".gpx")
		if err := exportgpx.WriteFile(outPath, lg.Name, coords); err != nil {
			return fmt.Errorf("export-gpx: %w", err)
		}
		fmt.Printf("wrote %s (%d points)\n", outPath, len(coords))
		written++
	}

	if *tripName != "" && written == 0 {
		return fmt.Errorf("export-gpx: no trip named %q has coordinates", *tripName)
	}

	fmt.Printf("done: %d trip(s) exported to %s\n", written, *outDir)
	return nil
}

// exportSlugify turns a trip name into a filesystem-safe basename. Trip
// names are unique (trips.name has a UNIQUE constraint), so this never
// needs to disambiguate collisions the way convert-csx's slugify once did.
func exportSlugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
