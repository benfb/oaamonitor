package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/ncruces/go-sqlite3/driver"
)

// DB wraps the sql.DB connection and provides methods for database operations
type DB struct {
	*sql.DB
}

// Open opens a SQLite database with the repository's standard pragmas.
func Open(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dsn(dbPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	return db, nil
}

// New creates and returns a new DB instance
func New(dbPath string) (*DB, error) {
	// Ensure data directory exists
	dataDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %v", err)
	}

	db, err := Open(dbPath)
	if err != nil {
		return nil, err
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	log.Println("Database connection established")
	return &DB{db}, nil
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

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_date ON outs_above_average(date)`,
		`CREATE INDEX IF NOT EXISTS idx_team_lower ON outs_above_average(LOWER(team))`,
		`CREATE INDEX IF NOT EXISTS idx_name ON outs_above_average(last_name, first_name)`,
		`CREATE INDEX IF NOT EXISTS idx_team_date ON outs_above_average(LOWER(team), date)`,
	}

	for _, indexSQL := range indexes {
		if _, err := db.Exec(indexSQL); err != nil {
			return fmt.Errorf("failed to create index: %v", err)
		}
	}

	return nil
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

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// GetDatabaseSize returns the size of the database file in a human-readable format
func GetDatabaseSize(dbPath string) (string, error) {
	fileInfo, err := os.Stat(dbPath)
	if err != nil {
		return "", err
	}
	size := fileInfo.Size()
	return fmt.Sprintf("%.2f MB", float64(size)/1024/1024), nil
}

func dsn(dbPath string) string {
	return "file:" + dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
}
