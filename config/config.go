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
	downloadDatabase := os.Getenv("DOWNLOAD_DATABASE")
	if downloadDatabase == "" {
		downloadDatabase = "false"
	}
	downloadDatabaseBool, err := strconv.ParseBool(downloadDatabase)
	if err != nil {
		log.Fatal(err)
	}
	uploadDatabase := os.Getenv("UPLOAD_DATABASE")
	if uploadDatabase == "" {
		uploadDatabase = "false"
	}
	uploadDatabaseBool, err := strconv.ParseBool(uploadDatabase)
	if err != nil {
		log.Fatal(err)
	}

	return &Config{
		DB:               &sql.DB{},
		DatabasePath:     dbPath,
		DownloadDatabase: downloadDatabaseBool,
		RefreshRate:      refreshRateInt,
		UploadDatabase:   uploadDatabaseBool,
	}
}
