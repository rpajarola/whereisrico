// Command whereisricod serves the whereisrico web app: the static
// MapLibre-based frontend and a live GeoJSON API computed from the
// database, replacing the old cron-generated static KML file.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/rpajarola/whereisrico/internal/geojson"
	"github.com/rpajarola/whereisrico/internal/storage"
)

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	dbPath := flag.String("db", "data/whereisrico.db", "path to the sqlite database")
	staticDir := flag.String("static", "web/static", "directory of static frontend assets to serve")
	flag.Parse()

	db, err := storage.Open(*dbPath)
	if err != nil {
		log.Fatalf("whereisricod: open db: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/geojson", geojsonHandler(db))
	mux.HandleFunc("/healthz", healthzHandler)
	mux.Handle("/", http.FileServer(http.Dir(*staticDir)))

	log.Printf("whereisricod: listening on %s (db=%s, static=%s)", *addr, *dbPath, *staticDir)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("whereisricod: %v", err)
	}
}

func geojsonHandler(db *storage.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		fc, err := geojson.Build(ctx, db, geojson.Options{})
		if err != nil {
			log.Printf("whereisricod: build geojson: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/geo+json")
		w.Header().Set("Cache-Control", "no-cache")
		if err := json.NewEncoder(w).Encode(fc); err != nil {
			log.Printf("whereisricod: encode geojson response: %v", err)
		}
	}
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
