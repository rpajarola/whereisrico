# whereisrico

Displays travel history on a world map: GPS tracks (GPX) and hand-authored
trip/flight logs (`.textproto`, see `proto/trip/v1/trip.proto`) are ingested
into a SQLite database and served as GeoJSON to a MapLibre GL globe
frontend.

The original Python/KML implementation this was ported from is archived in
the private `whereisrico-data` repo's `old/` directory (see Data below),
not in this repo.

## Data

`data/trips/`, `data/gpx/`, and `data/whereisrico.db` are personal travel
data and are not part of this repo's git history -- they're gitignored
here and live in a separate private repo
([rpajarola/whereisrico-data](https://github.com/rpajarola/whereisrico-data)).
Clone that repo and copy its `data/` into this one to populate a local
checkout:

```
git clone git@github.com:rpajarola/whereisrico-data.git /tmp/whereisrico-data
cp -r /tmp/whereisrico-data/data/trips /tmp/whereisrico-data/data/gpx data/
cp /tmp/whereisrico-data/data/whereisrico.db data/
```

## Build

```
go build ./...
```

## Run

```
# Ingest GPX / .textproto files from data/ (and data/trips/) into the db,
# skipping files unchanged since the last run.
go run ./cmd/whereisricoctl ingest --db data/whereisrico.db --dir data

# Convert a Google Maps Timeline export (Timeline.json, exported from the
# Maps app) to GPX for one trip: the date range comes from the trip in the
# db, points during its flights are dropped, and the track is thinned to
# roughly one point per 20 min / 20 km. Writes data/gpx/timeline-<trip>.gpx
# for the next ingest. Use --from/--to/--out instead of --trip for an
# arbitrary range; see --help for the thinning knobs.
go run ./cmd/whereisricoctl convert-timeline --in Timeline.json \
    --trip "Australia 2024"

# Serve the frontend and GeoJSON API.
go run ./cmd/whereisricod --db data/whereisrico.db --static web/static --addr :8080
```

Then open `http://localhost:8080`.

## Test

```
go test ./...
```
