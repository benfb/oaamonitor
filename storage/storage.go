package storage

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
)

const (
	defaultDatabaseBucket = "oaamonitor"
	defaultDatabaseKey    = "oaamonitor.db"
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

func databaseObjectLocation() (string, string) {
	bucket := os.Getenv("DATABASE_STORAGE_BUCKET")
	if bucket == "" {
		bucket = defaultDatabaseBucket
	}

	key := os.Getenv("DATABASE_STORAGE_KEY")
	if key == "" {
		key = defaultDatabaseKey
	}

	return bucket, key
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
	bucket, key := databaseObjectLocation()
	resp, err := client.GetObject(ctx, bucket, key)
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
	if err := ValidateSQLite(tmpPath); err != nil {
		return fmt.Errorf("downloaded database is invalid: %w", err)
	}

	return os.Rename(tmpPath, dbPath)
}

// ValidateSQLite runs an integrity check against the database at path.
func ValidateSQLite(path string) error {
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

	bucket, key := databaseObjectLocation()

	// Keep a dated copy first so a bad refresh can't destroy the only copy of the history.
	if err := client.PutObject(ctx, bucket, backupKey(key, time.Now()), file, fileInfo.Size()); err != nil {
		log.Printf("Failed to upload database backup: %v", err)
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}

	err = client.PutObject(ctx, bucket, key, file, fileInfo.Size())
	if err != nil {
		log.Printf("Failed to upload database file: %v", err)
		return err
	}

	return nil
}

// backupKey returns the object key for the daily backup of the database stored at key.
func backupKey(key string, now time.Time) string {
	return path.Join("backups", now.UTC().Format("2006-01-02")+"-"+path.Base(key))
}
