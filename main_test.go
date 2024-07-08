package main

import (
	"os"
	"testing"
)

func TestGetDatabaseSize(t *testing.T) {
	// Create a temporary file for testing
	file, err := os.CreateTemp("", "test.db")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(file.Name())

	// Write some data to the file
	data := make([]byte, 10240) // 10 KB
	for i := range data {
		data[i] = 'a'
	}
	_, err = file.Write(data)
	if err != nil {
		t.Fatalf("Failed to write data to file: %v", err)
	}

	// Get the size of the file
	size, err := getDatabaseSize(file.Name())
	if err != nil {
		t.Fatalf("Failed to get database size: %v", err)
	}

	// Verify that the size is correct
	expectedSize := "0.01 MB"
	if size != expectedSize {
		t.Errorf("Unexpected database size. Got: %s, want: %s", size, expectedSize)
	}
}
