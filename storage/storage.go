package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
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

	// Download the SQLite database file
	resp, err := client.GetObject("oaamonitor", "oaamonitor.db")
	if err != nil {
		log.Printf("Failed to download database file: %v", err)
		return err
	}
	defer resp.Close()

	file, err := os.Create(dbPath)
	if err != nil {
		log.Printf("Failed to create database file: %v", err)
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp)
	if err != nil {
		log.Printf("Failed to save database file: %v", err)
		return err
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

	err = client.PutObject("oaamonitor", "oaamonitor.db", file, fileInfo.Size())
	if err != nil {
		log.Printf("Failed to upload database file: %v", err)
		return err
	}

	return nil
}
