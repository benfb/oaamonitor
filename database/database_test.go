package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesDatabaseFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "testdb.sqlite")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("expected database file to exist: %v", err)
	}
}

func TestOpenInvalidPath(t *testing.T) {
	if _, err := Open("/invalid_path/testdb.sqlite"); err == nil {
		t.Error("expected an error for invalid path, got none")
	}
}

func TestValidate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := EnsureSchema(db); err != nil {
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

func TestCompactIfFragmented(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	defer db.Close()
	if err := EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema returned error: %v", err)
	}

	for i := 0; i < 5000; i++ {
		if _, err := db.Exec(`INSERT INTO outs_above_average (player_id, full_name, team, date) VALUES (?, 'Some Longer Player Name', 'Some Team', '2026-01-01')`, i); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	if _, err := db.Exec(`DELETE FROM outs_above_average`); err != nil {
		t.Fatalf("delete: %v", err)
	}
	compacted, err := CompactIfFragmented(db)
	if err != nil || !compacted {
		t.Fatalf("CompactIfFragmented = %v, %v; want true, nil", compacted, err)
	}
	if compacted, _ := CompactIfFragmented(db); compacted {
		t.Fatal("second CompactIfFragmented should find nothing to do")
	}
}
