package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/benfb/oaamonitor/config"
	"github.com/benfb/oaamonitor/refresher"
	"github.com/benfb/oaamonitor/storage"
)

func main() {
	databasePath := flag.String("database", "", "path to the SQLite database file (defaults to DATABASE_PATH env or ./data/oaamonitor.db)")
	enableUpload := flag.Bool("upload", false, "upload the refreshed database using configured storage credentials")
	fromStorage := flag.Bool("from-storage", false, "pull the latest database from object storage instead of downloading Baseball Savant data")
	flag.Parse()

	cfg := config.NewConfig()
	cfg.DownloadDatabase = false

	if *databasePath != "" {
		cfg.DatabasePath = *databasePath
	}

	if err := ensureDir(cfg.DatabasePath); err != nil {
		log.Fatalf("Failed to prepare database directory: %v", err)
	}

	if *fromStorage {
		log.Printf("Downloading database from object storage into %s", cfg.DatabasePath)
		if err := storage.DownloadDatabase(context.Background(), cfg.DatabasePath); err != nil {
			log.Fatalf("Failed to download database from storage: %v", err)
		}
		log.Println("Database downloaded successfully")
		return
	}

	if *enableUpload {
		cfg.UploadDatabase = true
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
