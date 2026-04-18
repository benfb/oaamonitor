package models

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
)

// setupTestDB creates an in-memory database with test data
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	// Create the necessary tables
	_, err = db.Exec(`
    CREATE TABLE IF NOT EXISTS outs_above_average (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		player_id INTEGER,
		first_name TEXT,
		last_name TEXT,
		full_name TEXT,
		team TEXT,
		primary_position TEXT,
		oaa INTEGER,
		actual_success_rate REAL,
		estimated_success_rate REAL,
		diff_success_rate REAL,
		date DATE DEFAULT CURRENT_DATE,
		UNIQUE(player_id, date)
	);`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert test data
	_, err = db.Exec(`
    INSERT INTO outs_above_average
    (player_id, full_name, first_name, last_name, team, primary_position, oaa, date, actual_success_rate, estimated_success_rate, diff_success_rate)
    VALUES
    (1, 'John Doe', 'John', 'Doe', 'Yankees', 'SS', 5, '2023-08-01', 0.75, 0.70, 0.05),
    (1, 'John Doe', 'John', 'Doe', 'Yankees', 'SS', 7, '2023-08-15', 0.78, 0.71, 0.07),
    (2, 'Jane Smith', 'Jane', 'Smith', 'Red Sox', 'CF', 6, '2023-08-01', 0.82, 0.76, 0.06),
    (2, 'Jane Smith', 'Jane', 'Smith', 'Red Sox', 'CF', 4, '2023-08-15', 0.80, 0.75, 0.05),
    (3, 'Bob Johnson', 'Bob', 'Johnson', 'Dodgers', '1B', 3, '2023-08-01', 0.65, 0.62, 0.03),
    (3, 'Bob Johnson', 'Bob', 'Johnson', 'Dodgers', NULL, 2, '2023-08-15', 0.64, 0.61, 0.03),
    (4, 'Mike Brown', 'Mike', 'Brown', 'Padres', '2B', 1, '2022-08-01', 0.60, 0.59, 0.01);`)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	return db
}

func TestFetchTeamStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Test successful query
	stats, teamName, err := FetchTeamStats(db, "yankees", 2023)
	if err != nil {
		t.Errorf("FetchTeamStats returned error: %v", err)
	}
	if len(stats) != 2 {
		t.Errorf("Expected 2 stats, got %d", len(stats))
	}
	if teamName != "Yankees" {
		t.Errorf("Expected team name 'Yankees', got '%s'", teamName)
	}

	// Test no data found case
	stats, teamName, err = FetchTeamStats(db, "padres", 2023)
	if err != nil {
		t.Errorf("FetchTeamStats returned error: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("Expected 0 stats, got %d", len(stats))
	}
	if teamName != "Padres" {
		t.Errorf("Expected team name 'Padres', got '%s'", teamName)
	}
}

func TestMapStatsByPlayerID(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)

	stats := []Stat{
		{PlayerID: 1, Name: "John Doe", Position: "SS", OAA: 3, Date: yesterday},
		{PlayerID: 1, Name: "John Doe", Position: "SS", OAA: 5, Date: now},
		{PlayerID: 2, Name: "Jane Smith", Position: "CF", OAA: 7, Date: now},
		{PlayerID: 3, Name: "Bob Johnson", Position: "N/A", OAA: 2, Date: now},
		{PlayerID: 3, Name: "Bob Johnson", Position: "1B", OAA: 1, Date: yesterday},
	}

	result := MapStatsByPlayerID(stats)

	if len(result) != 3 {
		t.Errorf("Expected 3 players, got %d", len(result))
	}

	// Jane should be first with OAA 7
	if result[0].PlayerID != 2 || result[0].LatestOAA != 7 {
		t.Errorf("Expected player 2 with OAA 7, got player %d with OAA %d",
			result[0].PlayerID, result[0].LatestOAA)
	}

	// John should be second with OAA 5
	if result[1].PlayerID != 1 || result[1].LatestOAA != 5 {
		t.Errorf("Expected player 1 with OAA 5, got player %d with OAA %d",
			result[1].PlayerID, result[1].LatestOAA)
	}

	// John should have 2 history points
	if len(result[1].OAAHistory) != 2 {
		t.Errorf("Expected 2 history points for player 1, got %d", len(result[1].OAAHistory))
	}

	// Bob should be third with OAA 2
	if result[2].PlayerID != 3 || result[2].LatestOAA != 2 {
		t.Errorf("Expected player 3 with OAA 2, got player %d with OAA %d",
			result[2].PlayerID, result[2].LatestOAA)
	}

	// Position should be taken from non-N/A value
	if result[2].Position != "1B" {
		t.Errorf("Expected position '1B', got '%s'", result[2].Position)
	}
}

func TestGetTeamAbbreviation(t *testing.T) {
	tests := []struct {
		teamName string
		expected string
	}{
		{"Yankees", "NYY"},
		{"Red Sox", "BOS"},
		{"Dodgers", "LAD"},
		{"Unknown Team", ""},
	}

	for _, tt := range tests {
		result := GetTeamAbbreviation(tt.teamName)
		if result != tt.expected {
			t.Errorf("GetTeamAbbreviation(%s) = %s, expected %s",
				tt.teamName, result, tt.expected)
		}
	}
}

func TestFetchPlayers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	players, err := FetchPlayers(db)
	if err != nil {
		t.Errorf("FetchPlayers returned error: %v", err)
	}

	if len(players) != 4 {
		t.Errorf("Expected 4 players, got %d", len(players))
	}

	// Check first player
	if players[0].ID != 4 || players[0].Name != "Mike Brown" {
		t.Errorf("Expected player (4, 'Mike Brown'), got (%d, '%s')",
			players[0].ID, players[0].Name)
	}
}

func TestFetchPlayers_DeduplicatesAfterNameCorrection(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert an older row for player 1 with a different (pre-correction) name.
	_, err := db.Exec(`
		INSERT INTO outs_above_average
		(player_id, full_name, first_name, last_name, team, primary_position, oaa, date, actual_success_rate, estimated_success_rate, diff_success_rate)
		VALUES (1, 'Johnathan Doe', 'Johnathan', 'Doe', 'Yankees', 'SS', 4, '2022-01-01', 0.75, 0.70, 0.05)
	`)
	if err != nil {
		t.Fatalf("Failed to insert corrected name row: %v", err)
	}

	players, err := FetchPlayers(db)
	if err != nil {
		t.Fatalf("FetchPlayers returned error: %v", err)
	}

	seen := make(map[int]int)
	for _, p := range players {
		seen[p.ID]++
	}
	for id, count := range seen {
		if count > 1 {
			t.Errorf("player_id %d appears %d times, expected 1", id, count)
		}
	}

	for _, p := range players {
		if p.ID == 1 && p.Name != "John Doe" {
			t.Errorf("expected player 1 to have most recent name 'John Doe', got %q", p.Name)
		}
	}
}

func TestFetchPlayerStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Test successful query
	stats, name, position, err := FetchPlayerStats(db, 1, 2023)
	if err != nil {
		t.Errorf("FetchPlayerStats returned error: %v", err)
	}
	if len(stats) != 2 {
		t.Errorf("Expected 2 stats, got %d", len(stats))
	}
	if name != "John Doe" {
		t.Errorf("Expected name 'John Doe', got '%s'", name)
	}
	if position != "SS" {
		t.Errorf("Expected position 'SS', got '%s'", position)
	}

	// Test no data for specified season but player exists
	stats, name, position, err = FetchPlayerStats(db, 4, 2023)
	if err != nil {
		t.Errorf("FetchPlayerStats returned error: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("Expected 0 stats, got %d", len(stats))
	}
	if name != "Mike Brown" {
		t.Errorf("Expected name 'Mike Brown', got '%s'", name)
	}
}

func TestFetchTeams(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	teams, err := FetchTeams(db)
	if err != nil {
		t.Errorf("FetchTeams returned error: %v", err)
	}

	if len(teams) != 4 {
		t.Errorf("Expected 4 teams, got %d", len(teams))
	}

	// Teams should be sorted alphabetically
	expectedTeams := []string{"Dodgers", "Padres", "Red Sox", "Yankees"}
	for i, team := range teams {
		if team != expectedTeams[i] {
			t.Errorf("Expected team[%d] to be '%s', got '%s'",
				i, expectedTeams[i], team)
		}
	}
}

func TestFetchPlayerDifferences(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add recent dates for testing differences
	_, err := db.Exec(`
    UPDATE outs_above_average SET date = date('now') WHERE date = '2023-08-15';
    UPDATE outs_above_average SET date = date('now', '-1 day') WHERE date = '2023-08-01';
    `)
	if err != nil {
		t.Fatalf("Failed to update test data: %v", err)
	}

	diffs, err := FetchPlayerDifferences(db, 10)
	if err != nil {
		t.Errorf("FetchPlayerDifferences returned error: %v", err)
	}

	// We should have differences for the players
	if len(diffs) < 1 {
		t.Errorf("Expected at least 1 difference, got %d", len(diffs))
	}

	// John Doe's OAA should have increased by 2
	foundJohn := false
	for _, diff := range diffs {
		if diff.PlayerID == 1 && diff.Name == "John Doe" {
			foundJohn = true
			if diff.Difference != 2 {
				t.Errorf("Expected John Doe's difference to be 2, got %d", diff.Difference)
			}
		}
	}

	if !foundJohn {
		t.Errorf("Expected to find John Doe in the results")
	}
}

func TestFetchPlayerDifferences_FallbackWithDateGap(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE outs_above_average (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			player_id INTEGER, first_name TEXT, last_name TEXT, full_name TEXT,
			team TEXT, primary_position TEXT, oaa INTEGER,
			actual_success_rate REAL, estimated_success_rate REAL, diff_success_rate REAL,
			date DATE DEFAULT CURRENT_DATE, UNIQUE(player_id, date)
		)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Three snapshots with a 2-day gap between each (not consecutive calendar days).
	// today == 4-days-ago OAA (so the primary query finds count==0, triggering fallback).
	// 4-days-ago != 6-days-ago OAA (so there are changes in the fallback period).
	_, err = db.Exec(`
		INSERT INTO outs_above_average (player_id, full_name, first_name, last_name, team, primary_position, oaa, date, actual_success_rate, estimated_success_rate, diff_success_rate) VALUES
		(1, 'John Doe', 'John', 'Doe', 'Yankees', 'SS', 5, date('now'),           0.8, 0.7, 0.1),
		(1, 'John Doe', 'John', 'Doe', 'Yankees', 'SS', 5, date('now', '-2 days'), 0.8, 0.7, 0.1),
		(1, 'John Doe', 'John', 'Doe', 'Yankees', 'SS', 2, date('now', '-4 days'), 0.7, 0.6, 0.1)
	`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	diffs, err := FetchPlayerDifferences(db, 10)
	if err != nil {
		t.Fatalf("FetchPlayerDifferences returned error: %v", err)
	}
	if len(diffs) == 0 {
		t.Fatal("fallback returned no differences; calendar-arithmetic bug may have regressed")
	}
	if diffs[0].PlayerID != 1 || diffs[0].Difference != 3 {
		t.Errorf("expected player 1 with difference 3, got player %d difference %d", diffs[0].PlayerID, diffs[0].Difference)
	}
}

func TestFetchNDayTrends(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Set dates to be within the last 7 days
	_, err := db.Exec(`
    UPDATE outs_above_average SET date = date('now') WHERE date = '2023-08-15';
    UPDATE outs_above_average SET date = date('now', '-5 day') WHERE date = '2023-08-01';
    `)
	if err != nil {
		t.Fatalf("Failed to update test data: %v", err)
	}

	trends, err := FetchNDayTrends(db, 7, 0) // Set threshold to 0 to get all differences
	if err != nil {
		t.Errorf("FetchNDayTrends returned error: %v", err)
	}

	if len(trends) < 1 {
		t.Errorf("Expected at least 1 trend, got %d", len(trends))
	}
}

func TestFetchSeasons(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	seasons, err := FetchSeasons(db)
	if err != nil {
		t.Errorf("FetchSeasons returned error: %v", err)
	}

	if len(seasons) != 2 {
		t.Errorf("Expected 2 seasons, got %d", len(seasons))
	}

	// 2023 should be first (descending order)
	if seasons[0] != 2023 {
		t.Errorf("Expected first season to be 2023, got %d", seasons[0])
	}
}
