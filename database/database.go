package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps the sql.DB connection and provides methods for database operations
type DB struct {
	*sql.DB
}

// New creates and returns a new DB instance
func New(dbPath string) (*DB, error) {
	// Ensure data directory exists
	dataDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	log.Println("Database connection established")
	return &DB{db}, nil
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
