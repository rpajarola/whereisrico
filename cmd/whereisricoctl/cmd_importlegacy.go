package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/rpajarola/whereisrico/internal/legacyimport"
	"github.com/rpajarola/whereisrico/internal/storage"
)

func runImportLegacy(args []string) error {
	fs := flag.NewFlagSet("import-legacy", flag.ExitOnError)
	oldDBPath := fs.String("old-db", "old/whereisrico.db", "path to the old Python system's sqlite database")
	newDBPath := fs.String("new-db", "data/whereisrico.db", "path to the new sqlite database")
	excludeGlob := fs.String("exclude-source-glob", "", `SQL LIKE pattern; old rows whose source matches it are skipped (e.g. "%manual.csx%" once that data has been converted to .textproto and ingested separately)`)
	if err := fs.Parse(args); err != nil {
		return err
	}

	newDB, err := storage.Open(*newDBPath)
	if err != nil {
		return fmt.Errorf("import-legacy: open new db: %w", err)
	}
	defer newDB.Close()

	result, err := legacyimport.Import(context.Background(), *oldDBPath, newDB, legacyimport.Options{
		ExcludeSourceGlob: *excludeGlob,
	})
	if err != nil {
		return fmt.Errorf("import-legacy: %w", err)
	}

	fmt.Printf("done: coords %d/%d inserted, trips %d/%d inserted\n",
		result.CoordsInserted, result.CoordsSeen, result.TripsInserted, result.TripsSeen)
	return nil
}
