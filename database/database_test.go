package database

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestGetDatabaseSize(t *testing.T) {
	// Create a temporary file to simulate the database file
	tempFile, err := os.CreateTemp("", "testdb*.sqlite")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write some data to the file to give it a size
	data := []byte("This is some test data.")
	if _, err := tempFile.Write(data); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	// Get the size of the database file
	size, err := GetDatabaseSize(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to get database size: %v", err)
	}

	// Check if the size is as expected
	expectedSize := fmt.Sprintf("%.2f MB", float64(len(data))/1024/1024)
	if size != expectedSize {
		t.Errorf("Expected size %s, got %s", expectedSize, size)
	}
}

func TestGetDatabaseSize_FileNotExist(t *testing.T) {
	// Try to get the size of a non-existent file
	_, err := GetDatabaseSize("non_existent_file.sqlite")
	if err == nil {
		t.Error("Expected an error for non-existent file, but got none")
	}
}
func TestNew(t *testing.T) {
	// Create a temporary directory to simulate the database directory
	tempDir, err := os.MkdirTemp("", "testdbdir")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Define the path for the temporary database file
	dbPath := filepath.Join(tempDir, "testdb.sqlite")

	// Create a new database instance
	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create new database: %v", err)
	}
	defer db.Close()

	// Check if the database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Expected database file to be created, but it does not exist")
	}
}

func TestNew_InvalidPath(t *testing.T) {
	// Define an invalid path for the database file
	invalidPath := "/invalid_path/testdb.sqlite"

	// Try to create a new database instance with the invalid path
	_, err := New(invalidPath)
	if err == nil {
		t.Error("Expected an error for invalid path, but got none")
	}
}
