package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// NeedsIngest reports whether path has not yet been ingested, or has
// changed size or modification time since it last was.
func (db *DB) NeedsIngest(ctx context.Context, path string, size int64, modTime time.Time) (bool, error) {
	var gotSize, gotModTime int64
	row := db.sql.QueryRowContext(ctx,
		`SELECT size_bytes, mod_time FROM ingested_files WHERE path = ?`, path)
	err := row.Scan(&gotSize, &gotModTime)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return true, nil
	case err != nil:
		return false, fmt.Errorf("storage: needs ingest: %w", err)
	}
	return gotSize != size || gotModTime != modTime.Unix(), nil
}

// MarkIngested records that path (with the given size and modification
// time) has been ingested, so a later NeedsIngest call for the same
// unchanged file returns false.
func (db *DB) MarkIngested(ctx context.Context, path string, size int64, modTime time.Time) error {
	_, err := db.sql.ExecContext(ctx,
		`INSERT INTO ingested_files (path, size_bytes, mod_time, ingested_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(path) DO UPDATE SET
		   size_bytes = excluded.size_bytes,
		   mod_time = excluded.mod_time,
		   ingested_at = excluded.ingested_at`,
		path, size, modTime.Unix(), time.Now().Unix())
	if err != nil {
		return fmt.Errorf("storage: mark ingested: %w", err)
	}
	return nil
}
