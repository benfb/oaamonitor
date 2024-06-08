package storage

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func DownloadDatabase(dbPath string) error {
	sdkConfig, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Printf("Failed to load SDK configuration: %v", err)
		return err
	}
	// Create S3 service client
	svc := s3.NewFromConfig(sdkConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://fly.storage.tigris.dev")
		o.Region = "auto"
	})
	// Download the SQLite database file
	file, err := os.Create(dbPath)
	if err != nil {
		log.Printf("Failed to create database file: %v", err)
		return err
	}
	defer file.Close()
	resp, err := svc.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String("oaamonitor"),
		Key:    aws.String("oaamonitor.db"),
	})
	if err != nil {
		log.Printf("Failed to download database file: %v", err)
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		log.Printf("Failed to save database file: %v", err)
		return err
	}
	return nil
}

// Upload the SQLite database to Tigris Fly Storage
func UploadDatabase(dbPath string) error {
	sdkConfig, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Printf("Failed to load SDK configuration: %v", err)
		return err
	}

	// Create S3 service client
	svc := s3.NewFromConfig(sdkConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://fly.storage.tigris.dev")
		o.Region = "auto"
	})

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

	_, err = svc.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:        aws.String("oaamonitor"),
		Key:           aws.String("oaamonitor.db"),
		Body:          file,
		ContentLength: aws.Int64(fileInfo.Size()),
	})
	if err != nil {
		log.Printf("Failed to upload database file: %v", err)
		return err
	}

	return nil
}
