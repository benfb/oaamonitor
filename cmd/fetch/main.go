package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/benfb/oaamonitor/config"
	"github.com/benfb/oaamonitor/database"
	"github.com/benfb/oaamonitor/refresher"
)

func main() {
	databasePath := flag.String("database", "", "path to the SQLite database file (defaults to DATABASE_PATH env or ./data/oaamonitor.db)")
	flag.Parse()

	cfg := config.NewConfig()
	if *databasePath != "" {
		cfg.DatabasePath = *databasePath
	}

	if err := ensureDir(cfg.DatabasePath); err != nil {
		log.Fatalf("Failed to prepare database directory: %v", err)
	}

	// Refuse to append to a database that is already damaged.
	if _, err := os.Stat(cfg.DatabasePath); err == nil {
		if err := database.Validate(cfg.DatabasePath); err != nil {
			log.Fatalf("Existing database is invalid: %v", err)
		}
	}

	log.Printf("Refreshing Outs Above Average data into %s", cfg.DatabasePath)
	if err := refresher.GetLatestOAA(context.Background(), cfg); err != nil {
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
