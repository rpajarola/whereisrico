# whereisrico

Displays travel history on a world map: GPS tracks (GPX) and hand-authored
trip/flight logs (`.textproto`, see `proto/trip/v1/trip.proto`) are ingested
into a SQLite database and served as GeoJSON to a MapLibre GL globe
frontend.

`old/` contains the original Python/KML implementation, kept for historical
reference only.

## Build

```
go build ./...
```

## Run

```
# Ingest GPX / .textproto files from data/ (and data/trips/) into the db,
# skipping files unchanged since the last run.
go run ./cmd/whereisricoctl ingest --db data/whereisrico.db --dir data

# Serve the frontend and GeoJSON API.
go run ./cmd/whereisricod --db data/whereisrico.db --static web/static --addr :8080
```

Then open `http://localhost:8080`.

## Test

```
go test ./...
```
