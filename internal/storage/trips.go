package storage

import (
	"context"
	"fmt"
	"time"
)

// Trip marks the start of a named trip leg. Coordinates belong to the leg
// whose timestamp is the closest preceding trip timestamp.
type Trip struct {
	ID        int64
	Timestamp time.Time
	Name      string
	Source    string
}

// InsertTrip inserts t, ignoring it if a trip with the same name or
// timestamp already exists. inserted reports whether a new row was written.
func (db *DB) InsertTrip(ctx context.Context, t Trip) (inserted bool, err error) {
	res, err := db.sql.ExecContext(ctx,
		`INSERT OR IGNORE INTO trips (timestamp, name, source) VALUES (?, ?, ?)`,
		t.Timestamp.Unix(), t.Name, t.Source)
	if err != nil {
		return false, fmt.Errorf("storage: insert trip: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("storage: insert trip rows affected: %w", err)
	}
	return n > 0, nil
}

// Trips returns all trips ordered by timestamp ascending.
func (db *DB) Trips(ctx context.Context) ([]Trip, error) {
	rows, err := db.sql.QueryContext(ctx,
		`SELECT id, timestamp, name, source FROM trips ORDER BY timestamp`)
	if err != nil {
		return nil, fmt.Errorf("storage: trips: %w", err)
	}
	defer rows.Close()

	var out []Trip
	for rows.Next() {
		var t Trip
		var ts int64
		if err := rows.Scan(&t.ID, &ts, &t.Name, &t.Source); err != nil {
			return nil, fmt.Errorf("storage: scan trip: %w", err)
		}
		t.Timestamp = time.Unix(ts, 0).UTC()
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: iterate trips: %w", err)
	}
	return out, nil
}
