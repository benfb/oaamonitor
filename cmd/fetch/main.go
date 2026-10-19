package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/benfb/oaamonitor/database"
	"github.com/benfb/oaamonitor/refresher"
)

func main() {
	defaultDatabase := "./data/oaamonitor.db"
	if path := os.Getenv("DATABASE_PATH"); path != "" {
		defaultDatabase = path
	}
	databasePath := flag.String("database", defaultDatabase, "path to the SQLite database file (defaults to DATABASE_PATH env or ./data/oaamonitor.db)")
	timeout := flag.Duration("timeout", 30*time.Second, "timeout for the Baseball Savant request")
	flag.Parse()

	if err := ensureDir(*databasePath); err != nil {
		log.Fatalf("Failed to prepare database directory: %v", err)
	}

	// Refuse to append to a database that is already damaged.
	if _, err := os.Stat(*databasePath); err == nil {
		if err := database.Validate(*databasePath); err != nil {
			log.Fatalf("Existing database is invalid: %v", err)
		}
	}

	log.Printf("Refreshing Outs Above Average data into %s", *databasePath)
	if err := refresher.GetLatestOAA(context.Background(), *databasePath, *timeout); err != nil {
		log.Fatalf("Failed to refresh data: %v", err)
	}
	log.Println("Database refreshed successfully")
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
