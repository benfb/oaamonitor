package main

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestValidateOutputDir(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"public", false},
		{"out/site", false},
		{"", true},
		{".", true},
		{"/", true},
		{"/etc", true},
		{"/Users/ben/public", true},
		{"../escape", true},
	}
	for _, tt := range tests {
		err := validateOutputDir(tt.path)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateOutputDir(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
		}
	}
}

func TestCopyAssetsShipsOnlyMinifiedBuilds(t *testing.T) {
	src := fstest.MapFS{
		"app.js":           {Data: []byte("full")},
		"app.min.js":       {Data: []byte("min")},
		"solo.js":          {Data: []byte("solo")},
		"nested/x.css":     {Data: []byte("x")},
		"nested/x.min.css": {Data: []byte("x")},
	}
	dst := t.TempDir()
	if err := copyAssets(src, dst); err != nil {
		t.Fatalf("copyAssets returned error: %v", err)
	}
	for name, want := range map[string]bool{"app.js": false, "app.min.js": true, "solo.js": true, "nested/x.css": false, "nested/x.min.css": true} {
		_, err := os.Stat(filepath.Join(dst, name))
		if (err == nil) != want {
			t.Errorf("%s exists = %v, want %v", name, err == nil, want)
		}
	}
}
