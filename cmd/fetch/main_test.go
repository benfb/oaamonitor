package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDirCreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "oaamonitor.db")
	if err := ensureDir(path); err != nil {
		t.Fatalf("ensureDir returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("expected parent directory to exist: %v", err)
	}
}
