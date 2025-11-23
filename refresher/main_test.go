package refresher

import (
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePercentage(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"50%", 0.5},
		{"25%", 0.25},
		{"100%", 1},
		{"0%", 0},
		{"-50%", -0.5},
		{"-25%", -0.25},
		{"-100%", -1},
		{"-0%", 0},
	}

	for _, test := range tests {
		result, err := parsePercentage(test.input)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if result != test.expected {
			t.Errorf("Unexpected result. Got: %f, want: %f", result, test.expected)
		}
	}
}
func TestRemoveBOM(t *testing.T) {
	input := strings.NewReader("\xEF\xBB\xBFHello, World!")
	expected := "Hello, World!"

	result := removeBOM(input)
	output, err := io.ReadAll(result)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if string(output) != expected {
		t.Errorf("Unexpected result. Got: %s, want: %s", string(output), expected)
	}
}

func TestProcessCSV_RollsBackOnReadError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Construct a CSV with a valid row followed by a malformed row that triggers a read error.
	csvContent := strings.Join([]string{
		"Name,PlayerID,Team,Col3,PrimaryPosition,Col5,OAA,Col7,Col8,Col9,Col10,Col11,Col12,ActualSuccessRate,EstimatedSuccessRate,DiffSuccessRate",
		"Smith, John,123,Blue Jays,x,SS,x,5,x,x,x,x,x,x,90%,80%,10%",
		"\"Unclosed,456,Team,x,1B,x,2,x,x,x,x,x,x,85%,70%,15%", // malformed quoted field
	}, "\n")

	tmpFile, err := os.CreateTemp(dir, "data*.csv")
	if err != nil {
		t.Fatalf("failed to create temp csv: %v", err)
	}
	if _, err := tmpFile.WriteString(csvContent); err != nil {
		t.Fatalf("failed to write csv: %v", err)
	}
	tmpFile.Close()

	if err := processCSV(context.Background(), tmpFile.Name(), dbPath); err == nil {
		t.Fatalf("expected processing to fail due to malformed row")
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM outs_above_average").Scan(&count); err != nil {
		t.Fatalf("failed to count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero rows after rollback, got %d", count)
	}
}
