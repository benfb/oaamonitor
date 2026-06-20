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

func TestProcessCSV_RollsBackOnParseError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Both rows have exactly 16 fields (quoted names so the CSV reader doesn't
	// count the comma as a separator). The second row has a non-numeric OAA,
	// so the CSV reader succeeds but processRecord fails mid-transaction.
	csvContent := strings.Join([]string{
		"Name,PlayerID,Team,Col3,PrimaryPosition,Col5,OAA,Col7,Col8,Col9,Col10,Col11,Col12,ActualSuccessRate,EstimatedSuccessRate,DiffSuccessRate",
		`"Smith, John",123,Blue Jays,x,SS,x,5,x,x,x,x,x,x,90%,80%,10%`,
		`"Doe, Jane",456,Red Sox,x,CF,x,not-a-number,x,x,x,x,x,x,85%,70%,15%`,
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
		t.Fatal("expected processing to fail on parse error")
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM outs_above_average").Scan(&count); err != nil {
		t.Fatalf("failed to count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected zero rows after rollback (first valid row must not commit), got %d", count)
	}
}

func TestProcessCSV_UsesHeaderNamesWhenColumnsAreReordered(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	csvContent := strings.Join([]string{
		"Col3,DiffSuccessRate,Name,EstimatedSuccessRate,PlayerID,Col11,ActualSuccessRate,OAA,Team,Col12,Col7,PrimaryPosition,Col8,Col9,Col10,Col5",
		`x,10%,"Smith, John",80%,123,x,90%,5,Blue Jays,x,x,SS,x,x,x,x`,
	}, "\n")

	tmpFile, err := os.CreateTemp(dir, "data*.csv")
	if err != nil {
		t.Fatalf("failed to create temp csv: %v", err)
	}
	if _, err := tmpFile.WriteString(csvContent); err != nil {
		t.Fatalf("failed to write csv: %v", err)
	}
	tmpFile.Close()

	if err := processCSV(context.Background(), tmpFile.Name(), dbPath); err != nil {
		t.Fatalf("processCSV returned error: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	var (
		playerID             int
		firstName            string
		lastName             string
		fullName             string
		team                 string
		primaryPosition      string
		oaa                  int
		actualSuccessRate    float64
		estimatedSuccessRate float64
		diffSuccessRate      float64
	)
	if err := db.QueryRow(`
		SELECT player_id, first_name, last_name, full_name, team, primary_position, oaa,
			actual_success_rate, estimated_success_rate, diff_success_rate
		FROM outs_above_average
	`).Scan(&playerID, &firstName, &lastName, &fullName, &team, &primaryPosition, &oaa, &actualSuccessRate, &estimatedSuccessRate, &diffSuccessRate); err != nil {
		t.Fatalf("failed to read inserted row: %v", err)
	}

	if playerID != 123 || firstName != "John" || lastName != "Smith" || fullName != "John Smith" {
		t.Fatalf("unexpected player identity: id=%d first=%q last=%q full=%q", playerID, firstName, lastName, fullName)
	}
	if team != "Blue Jays" || primaryPosition != "SS" || oaa != 5 {
		t.Fatalf("unexpected player fields: team=%q position=%q oaa=%d", team, primaryPosition, oaa)
	}
	if actualSuccessRate != 0.9 || estimatedSuccessRate != 0.8 || diffSuccessRate != 0.1 {
		t.Fatalf("unexpected success rates: actual=%f estimated=%f diff=%f", actualSuccessRate, estimatedSuccessRate, diffSuccessRate)
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

	db, err := sql.Open("sqlite3", dbPath)
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
