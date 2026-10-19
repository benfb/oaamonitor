package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"

	_ "github.com/ncruces/go-sqlite3/driver"
)

// Open opens a SQLite database with the repository's standard pragmas and verifies the connection.
func Open(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dsn(dbPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	return db, nil
}

// EnsureSchema creates the tables and indexes required by OAA Monitor.
func EnsureSchema(db *sql.DB) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS outs_above_average (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		player_id INTEGER,
		first_name TEXT,
		last_name TEXT,
		full_name TEXT,
		team TEXT,
		primary_position TEXT,
		oaa INTEGER,
		actual_success_rate REAL,
		estimated_success_rate REAL,
		diff_success_rate REAL,
		date DATE DEFAULT CURRENT_DATE,
		UNIQUE(player_id, date)
	);`
	if _, err := db.Exec(createTableSQL); err != nil {
		return err
	}

	// No secondary indexes: the build reads whole tables and UNIQUE(player_id, date)
	// covers the point lookups, so extra indexes only cost file size.
	return nil
}

// CompactIfFragmented runs VACUUM when free pages make up more than a tenth of the file,
// such as after dropping indexes, so the database stays small to upload and download.
func CompactIfFragmented(db *sql.DB) (bool, error) {
	var free, total int
	if err := db.QueryRow("PRAGMA freelist_count").Scan(&free); err != nil {
		return false, err
	}
	if err := db.QueryRow("PRAGMA page_count").Scan(&total); err != nil {
		return false, err
	}
	if total == 0 || free*10 <= total {
		return false, nil
	}
	if _, err := db.Exec("VACUUM"); err != nil {
		return false, err
	}
	return true, nil
}

// CheckpointWAL checkpoints pending WAL pages into the main database file.
func CheckpointWAL(dbPath string) error {
	db, err := Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return Checkpoint(db)
}

// Checkpoint checkpoints pending WAL pages on an open database connection.
func Checkpoint(db *sql.DB) error {
	_, err := db.Exec("PRAGMA wal_checkpoint(FULL)")
	return err
}

// Validate runs SQLite's quick_check against the database file at path without modifying it.
func Validate(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve database path: %w", err)
	}
	u := url.URL{Scheme: "file", Path: absPath, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		return err
	}
	defer db.Close()

	var result string
	if err := db.QueryRow("PRAGMA quick_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("quick_check returned %q", result)
	}
	return nil
}

func dsn(dbPath string) string {
	return "file:" + dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
}
