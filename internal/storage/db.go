// Package storage provides the SQLite-backed persistence layer for
// whereisrico: coordinates, trip legs, and a small ingestion-tracking table.
package storage

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// DB wraps a SQLite connection pool and exposes typed queries over the
// whereisrico schema.
type DB struct {
	sql *sql.DB
}

// Open opens (creating if necessary) the SQLite database at path and applies
// the schema. path may be ":memory:" for an in-process database, useful in
// tests.
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("storage: open %s: %w", path, err)
	}
	// SQLite only supports one writer at a time; a single connection avoids
	// SQLITE_BUSY errors from the pure-Go driver under concurrent access.
	sqlDB.SetMaxOpenConns(1)

	db := &DB{sql: sqlDB}
	if err := db.init(); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) init() error {
	if _, err := db.sql.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("storage: enable foreign keys: %w", err)
	}
	if _, err := db.sql.Exec(schemaSQL); err != nil {
		return fmt.Errorf("storage: apply schema: %w", err)
	}
	return nil
}

// Close closes the underlying database connection.
func (db *DB) Close() error {
	return db.sql.Close()
}

// WithTx runs fn inside a transaction, committing on success and rolling
// back if fn returns an error or panics.
func (db *DB) WithTx(ctx context.Context, fn func(*sql.Tx) error) (err error) {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin tx: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()
	err = fn(tx)
	return err
}
