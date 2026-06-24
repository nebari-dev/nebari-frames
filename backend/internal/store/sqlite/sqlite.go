// Package sqlite provides a SQLite-backed implementation of store.Repository.
// Callers must register the driver: import _ "modernc.org/sqlite".
package sqlite

import (
	"database/sql"
	"fmt"
	"strings"
)

// Open opens a SQLite database with WAL mode and recommended pragmas.
// SQLite is single-writer, so MaxOpenConns is pinned to 1.
func Open(path string) (*sql.DB, error) {
	dsn := path + "?" + strings.Join([]string{
		"_pragma=journal_mode=WAL",
		"_pragma=busy_timeout=5000",
		"_pragma=foreign_keys=ON",
		"_pragma=synchronous=NORMAL",
	}, "&")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite %s: %w", path, err)
	}
	return db, nil
}
