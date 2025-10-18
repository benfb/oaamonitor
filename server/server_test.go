package server

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/benfb/oaamonitor/config"
	"github.com/benfb/oaamonitor/database"
)

func TestNormalizeTeamName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Diamondbacks variations
		{"diamondbacks", "d-backs"},
		{"dbacks", "d-backs"},

		// Blue Jays variations
		{"blue-jays", "blue jays"},
		{"blue+jays", "blue jays"},
		{"bluejays", "blue jays"},

		// Red Sox variations
		{"red-sox", "red sox"},
		{"red+sox", "red sox"},
		{"redsox", "red sox"},

		// White Sox variations
		{"white-sox", "white sox"},
		{"white+sox", "white sox"},
		{"whitesox", "white sox"},

		// Teams that should pass through unchanged
		{"Yankees", "Yankees"},
		{"Dodgers", "Dodgers"},
		{"Cardinals", "Cardinals"},
		{"Mets", "Mets"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeTeamName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeTeamName(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseSeasonParam(t *testing.T) {
	// Create a test database
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Insert some test seasons
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS outs_above_average (
			id INTEGER PRIMARY KEY,
			date DATE
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert test data for multiple seasons
	testSeasons := []string{"2023-05-01", "2024-05-01", "2025-05-01"}
	for _, date := range testSeasons {
		_, err = db.Exec("INSERT INTO outs_above_average (date) VALUES (?)", date)
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	}

	cfg := &config.Config{}
	s := &Server{
		db:     db,
		config: cfg,
	}

	currentYear := time.Now().Year()

	tests := []struct {
		name           string
		seasonParam    string
		expectedSeason int
		expectError    bool
	}{
		{
			name:           "Valid season parameter",
			seasonParam:    "2024",
			expectedSeason: 2024,
			expectError:    false,
		},
		{
			name:           "Invalid season parameter (non-numeric)",
			seasonParam:    "abc",
			expectedSeason: currentYear, // Should default to current year
			expectError:    false,
		},
		{
			name:           "Empty season parameter (defaults to most recent)",
			seasonParam:    "",
			expectedSeason: 2025, // Most recent in test data
			expectError:    false,
		},
		{
			name:           "Another valid season",
			seasonParam:    "2023",
			expectedSeason: 2023,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test request with the season parameter
			req := httptest.NewRequest("GET", "/?season="+tt.seasonParam, nil)

			selectedSeason, seasons, err := s.parseSeasonParam(req)

			if tt.expectError && err == nil {
				t.Error("Expected error but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError {
				if selectedSeason != tt.expectedSeason {
					t.Errorf("Expected season %d, got %d", tt.expectedSeason, selectedSeason)
				}

				// Verify seasons list is populated
				if len(seasons) == 0 {
					t.Error("Expected seasons list to be populated")
				}

				// Verify seasons are in descending order
				for i := 1; i < len(seasons); i++ {
					if seasons[i-1] < seasons[i] {
						t.Error("Seasons should be in descending order")
					}
				}
			}
		})
	}
}
