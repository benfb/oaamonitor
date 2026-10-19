package refresher

import (
	"context"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strconv"
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

// savantHeader is the header row of Baseball Savant's OAA leaderboard CSV.
const savantHeader = `"last_name, first_name",player_id,display_team_name,year,primary_pos_formatted,fielding_runs_prevented,outs_above_average,outs_above_average_infront,outs_above_average_lateral_toward3bline,outs_above_average_lateral_toward1bline,outs_above_average_behind,outs_above_average_rhh,outs_above_average_lhh,actual_success_rate_formatted,adj_estimated_success_rate_formatted,diff_success_rate_formatted`

// savantRow builds a leaderboard row; name is "Last, First" and the remaining columns are fillers.
func savantRow(name, playerID, team, position, oaa string) string {
	return `"` + name + `",` + playerID + `,` + team + `,2026,` + position + `,4,` + oaa + `,1,2,1,1,3,2,90%,80%,10%`
}

// processTestCSV writes lines to a CSV file and runs processCSV against a fresh database.
func processTestCSV(t *testing.T, lines ...string) (*sql.DB, error) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	csvPath := filepath.Join(dir, "data.csv")
	if err := os.WriteFile(csvPath, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("failed to write csv: %v", err)
	}

	processErr := processCSV(context.Background(), csvPath, dbPath)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, processErr
}

func countRows(t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM outs_above_average").Scan(&count); err != nil {
		t.Fatalf("failed to count rows: %v", err)
	}
	return count
}

func TestProcessCSV_ParsesSavantRows(t *testing.T) {
	db, err := processTestCSV(t, savantHeader, savantRow("Smith, John", "123", "Blue Jays", "SS", "5"))
	if err != nil {
		t.Fatalf("processCSV returned error: %v", err)
	}

	var (
		playerID             int
		firstName, lastName  string
		fullName, team       string
		primaryPosition      string
		oaa                  int
		actualSuccessRate    float64
		estimatedSuccessRate float64
		diffSuccessRate      float64
		date                 string
	)
	if err := db.QueryRow(`
		SELECT player_id, first_name, last_name, full_name, team, primary_position, oaa,
			actual_success_rate, estimated_success_rate, diff_success_rate, date
		FROM outs_above_average
	`).Scan(&playerID, &firstName, &lastName, &fullName, &team, &primaryPosition, &oaa, &actualSuccessRate, &estimatedSuccessRate, &diffSuccessRate, &date); err != nil {
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
	if date != snapshotDate() {
		t.Fatalf("expected snapshot dated %s, got %s", snapshotDate(), date)
	}
}

func TestProcessCSV_UsesHeaderNamesWhenColumnsAreReordered(t *testing.T) {
	db, err := processTestCSV(t,
		`diff_success_rate_formatted,outs_above_average,player_id,"last_name, first_name",adj_estimated_success_rate_formatted,actual_success_rate_formatted,primary_pos_formatted,display_team_name`,
		`10%,5,123,"Smith, John",80%,90%,SS,Blue Jays`,
	)
	if err != nil {
		t.Fatalf("processCSV returned error: %v", err)
	}

	var fullName, team string
	var oaa int
	if err := db.QueryRow("SELECT full_name, team, oaa FROM outs_above_average").Scan(&fullName, &team, &oaa); err != nil {
		t.Fatalf("failed to read inserted row: %v", err)
	}
	if fullName != "John Smith" || team != "Blue Jays" || oaa != 5 {
		t.Fatalf("unexpected row: name=%q team=%q oaa=%d", fullName, team, oaa)
	}
}

func TestProcessCSV_FailsWhenColumnIsMissing(t *testing.T) {
	_, err := processTestCSV(t,
		`"last_name, first_name",player_id,display_team_name,primary_pos_formatted,oaa`,
		`"Smith, John",123,Blue Jays,SS,5`,
	)
	if err == nil || !strings.Contains(err.Error(), "outs_above_average") {
		t.Fatalf("expected missing-column error naming outs_above_average, got %v", err)
	}
}

func TestProcessCSV_SkipsAFewMalformedRows(t *testing.T) {
	lines := []string{savantHeader}
	for i := 1; i <= 40; i++ {
		lines = append(lines, savantRow("Smith, John", strconv.Itoa(i), "Blue Jays", "SS", "5"))
	}
	lines = append(lines, savantRow("Doe, Jane", "999", "Red Sox", "CF", "not-a-number"))

	db, err := processTestCSV(t, lines...)
	if err != nil {
		t.Fatalf("expected one bad row in 41 to be skipped, got error: %v", err)
	}
	if got := countRows(t, db); got != 40 {
		t.Fatalf("expected 40 rows imported, got %d", got)
	}
}

func TestProcessCSV_RollsBackWhenTooManyRowsAreMalformed(t *testing.T) {
	db, err := processTestCSV(t,
		savantHeader,
		savantRow("Smith, John", "123", "Blue Jays", "SS", "5"),
		savantRow("Doe, Jane", "456", "Red Sox", "CF", "not-a-number"),
	)
	if err == nil {
		t.Fatal("expected processing to fail when half the rows are malformed")
	}
	if got := countRows(t, db); got != 0 {
		t.Fatalf("expected zero rows after rollback (the valid row must not commit), got %d", got)
	}
}

func TestProcessCSV_RollsBackOnReadError(t *testing.T) {
	db, err := processTestCSV(t,
		savantHeader,
		savantRow("Smith, John", "123", "Blue Jays", "SS", "5"),
		`"Unclosed,456,Team,2026,1B,4,2,1,2,1,1,3,2,85%,70%,15%`, // malformed quoted field
	)
	if err == nil {
		t.Fatal("expected processing to fail due to malformed row")
	}
	if got := countRows(t, db); got != 0 {
		t.Fatalf("expected zero rows after rollback, got %d", got)
	}
}
