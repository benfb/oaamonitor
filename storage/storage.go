package storage

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"

	_ "github.com/ncruces/go-sqlite3/driver"
)

// getS3Client creates a new S3 client from environment variables
func getS3Client() (*S3Client, error) {
	accessKeyID := os.Getenv("AWS_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	region := os.Getenv("AWS_REGION")
	endpoint := os.Getenv("AWS_ENDPOINT_URL_S3")

	if accessKeyID == "" || secretAccessKey == "" {
		return nil, fmt.Errorf("AWS credentials not found in environment")
	}

	if region == "" {
		region = "auto"
	}

	if endpoint == "" {
		endpoint = "https://s3.amazonaws.com"
	}

	return NewS3Client(accessKeyID, secretAccessKey, region, endpoint), nil
}

func DownloadDatabase(ctx context.Context, dbPath string) error {
	client, err := getS3Client()
	if err != nil {
		log.Printf("Failed to create S3 client: %v", err)
		return err
	}
	return downloadDatabase(ctx, client, dbPath)
}

func downloadDatabase(ctx context.Context, client *S3Client, dbPath string) error {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}

	// Download the SQLite database file
	resp, err := client.GetObject(ctx, "oaamonitor", "oaamonitor.db")
	if err != nil {
		log.Printf("Failed to download database file: %v", err)
		return err
	}
	defer resp.Close()

	tmpFile, err := os.CreateTemp(filepath.Dir(dbPath), filepath.Base(dbPath)+".tmp-*")
	if err != nil {
		log.Printf("Failed to create temporary database file: %v", err)
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err = io.Copy(tmpFile, resp); err != nil {
		tmpFile.Close()
		log.Printf("Failed to save database file: %v", err)
		return err
	}
	if err = tmpFile.Sync(); err != nil {
		tmpFile.Close()
		log.Printf("Failed to sync database file: %v", err)
		return err
	}
	if err = tmpFile.Close(); err != nil {
		log.Printf("Failed to close database file: %v", err)
		return err
	}
	if err := validateSQLite(tmpPath); err != nil {
		return fmt.Errorf("downloaded database is invalid: %w", err)
	}

	return os.Rename(tmpPath, dbPath)
}

func validateSQLite(path string) error {
	u := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
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

// UploadDatabase uploads the SQLite database to object storage
func UploadDatabase(ctx context.Context, dbPath string) error {
	client, err := getS3Client()
	if err != nil {
		log.Printf("Failed to create S3 client: %v", err)
		return err
	}

	// Open the SQLite database file
	file, err := os.Open(dbPath)
	if err != nil {
		log.Printf("Failed to open database file: %v", err)
		return err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("Failed to get file info: %v", err)
		return err
	}

	err = client.PutObject(ctx, "oaamonitor", "oaamonitor.db", file, fileInfo.Size())
	if err != nil {
		log.Printf("Failed to upload database file: %v", err)
		return err
	}

	return nil
}
