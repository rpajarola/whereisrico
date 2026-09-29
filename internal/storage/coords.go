package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Coord is a single timestamped GPS position, optionally annotated with a
// human-readable message.
type Coord struct {
	ID        int64
	Timestamp time.Time
	Message   string
	Longitude float64
	Latitude  float64
	Source    string
}

// InsertCoord inserts c, ignoring it if a coordinate with the same
// (timestamp, longitude, latitude) already exists. inserted reports whether
// a new row was actually written.
func (db *DB) InsertCoord(ctx context.Context, c Coord) (inserted bool, err error) {
	res, err := db.sql.ExecContext(ctx,
		`INSERT OR IGNORE INTO coords (timestamp, message, longitude, latitude, source)
		 VALUES (?, ?, ?, ?, ?)`,
		c.Timestamp.Unix(), c.Message, c.Longitude, c.Latitude, c.Source)
	if err != nil {
		return false, fmt.Errorf("storage: insert coord: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("storage: insert coord rows affected: %w", err)
	}
	return n > 0, nil
}

// InsertCoords inserts cs in a single transaction, skipping duplicates as
// InsertCoord does. inserted is the count of rows actually written.
func (db *DB) InsertCoords(ctx context.Context, cs []Coord) (inserted int, err error) {
	err = db.WithTx(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx,
			`INSERT OR IGNORE INTO coords (timestamp, message, longitude, latitude, source)
			 VALUES (?, ?, ?, ?, ?)`)
		if err != nil {
			return fmt.Errorf("prepare insert coord: %w", err)
		}
		defer stmt.Close()

		for _, c := range cs {
			res, err := stmt.ExecContext(ctx, c.Timestamp.Unix(), c.Message, c.Longitude, c.Latitude, c.Source)
			if err != nil {
				return fmt.Errorf("insert coord %+v: %w", c, err)
			}
			n, err := res.RowsAffected()
			if err != nil {
				return fmt.Errorf("insert coord rows affected: %w", err)
			}
			inserted += int(n)
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("storage: insert coords: %w", err)
	}
	return inserted, nil
}

// CoordsBetween returns coordinates with start <= timestamp < end, ordered
// by timestamp ascending.
func (db *DB) CoordsBetween(ctx context.Context, start, end time.Time) ([]Coord, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, timestamp, message, longitude, latitude, source FROM coords
		 WHERE timestamp >= ? AND timestamp < ?
		 ORDER BY timestamp`,
		start.Unix(), end.Unix())
	if err != nil {
		return nil, fmt.Errorf("storage: coords between: %w", err)
	}
	defer rows.Close()
	return scanCoords(rows)
}

// CoordWithMessage returns coordinates in [start, end) that carry a
// non-empty message, ordered by timestamp ascending.
func (db *DB) CoordsWithMessageBetween(ctx context.Context, start, end time.Time) ([]Coord, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, timestamp, message, longitude, latitude, source FROM coords
		 WHERE message != '' AND timestamp >= ? AND timestamp < ?
		 ORDER BY timestamp`,
		start.Unix(), end.Unix())
	if err != nil {
		return nil, fmt.Errorf("storage: coords with message between: %w", err)
	}
	defer rows.Close()
	return scanCoords(rows)
}

func scanCoords(rows *sql.Rows) ([]Coord, error) {
	var out []Coord
	for rows.Next() {
		var c Coord
		var ts int64
		if err := rows.Scan(&c.ID, &ts, &c.Message, &c.Longitude, &c.Latitude, &c.Source); err != nil {
			return nil, fmt.Errorf("scan coord: %w", err)
		}
		c.Timestamp = time.Unix(ts, 0).UTC()
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate coords: %w", err)
	}
	return out, nil
}

// ErrNoCoords is returned by TimeRange when there are no coordinates
// matching the query.
var ErrNoCoords = errors.New("storage: no coordinates found")

// TimeRange returns the min and max coordinate timestamps strictly before
// the given instant.
func (db *DB) TimeRange(ctx context.Context, before time.Time) (min, max time.Time, err error) {
	var minTS, maxTS sql.NullInt64
	row := db.sql.QueryRowContext(ctx,
		`SELECT MIN(timestamp), MAX(timestamp) FROM coords WHERE timestamp < ?`, before.Unix())
	if err := row.Scan(&minTS, &maxTS); err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("storage: time range: %w", err)
	}
	if !minTS.Valid || !maxTS.Valid {
		return time.Time{}, time.Time{}, ErrNoCoords
	}
	return time.Unix(minTS.Int64, 0).UTC(), time.Unix(maxTS.Int64, 0).UTC(), nil
}
