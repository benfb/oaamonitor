package config

import (
	"database/sql"
	"log"
	"os"
	"strconv"
)

// Config is a struct that holds the configuration for the application.
type Config struct {
	DB               *sql.DB
	DatabasePath     string
	DownloadDatabase bool
	RefreshRate      int
	UploadDatabase   bool
}

// NewConfig returns a new Config struct.
func NewConfig() *Config {
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./data/oaamonitor.db"
	}
	refreshRate := os.Getenv("REFRESH_RATE")
	if refreshRate == "" {
		refreshRate = "3600"
	}
	refreshRateInt, err := strconv.Atoi(refreshRate)
	if err != nil {
		log.Fatal(err)
	}
	// Open the SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	return &Config{
		DB:               db,
		DatabasePath:     dbPath,
		DownloadDatabase: false,
		RefreshRate:      refreshRateInt,
		UploadDatabase:   false,
	}
}
