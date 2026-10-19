package database

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestValidate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := New(path)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := EnsureSchema(db.DB); err != nil {
		t.Fatalf("EnsureSchema returned error: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := Validate(path); err != nil {
		t.Fatalf("Validate rejected a valid database: %v", err)
	}

	bad := filepath.Join(t.TempDir(), "bad.db")
	if err := os.WriteFile(bad, []byte("not a SQLite database, just some text padding the file out"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Validate(bad); err == nil {
		t.Fatal("Validate accepted a non-SQLite file")
	}
}

func TestEnsureSchemaDropsLegacyIndexesAndCompacts(t *testing.T) {
	db, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer db.Close()
	if err := EnsureSchema(db.DB); err != nil {
		t.Fatalf("EnsureSchema returned error: %v", err)
	}

	if _, err := db.Exec(`CREATE INDEX idx_date ON outs_above_average(date)`); err != nil {
		t.Fatalf("create legacy index: %v", err)
	}
	for i := 0; i < 5000; i++ {
		if _, err := db.Exec(`INSERT INTO outs_above_average (player_id, full_name, team, date) VALUES (?, 'Some Longer Player Name', 'Some Team', '2026-01-01')`, i); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	if err := EnsureSchema(db.DB); err != nil {
		t.Fatalf("EnsureSchema returned error: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_date'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("legacy index still present: n=%d err=%v", n, err)
	}

	if _, err := db.Exec(`DELETE FROM outs_above_average`); err != nil {
		t.Fatalf("delete: %v", err)
	}
	compacted, err := CompactIfFragmented(db.DB)
	if err != nil || !compacted {
		t.Fatalf("CompactIfFragmented = %v, %v; want true, nil", compacted, err)
	}
	if compacted, _ := CompactIfFragmented(db.DB); compacted {
		t.Fatal("second CompactIfFragmented should find nothing to do")
	}
}
