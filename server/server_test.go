package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestHandleRefresh_MissingToken(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		DatabasePath: ":memory:",
	}

	// Create server struct directly to avoid template loading
	s := &Server{
		db:     db,
		config: cfg,
	}

	// Set a dummy token for the test
	os.Setenv("REFRESH_TOKEN", "test-token-123")
	defer os.Unsetenv("REFRESH_TOKEN")

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	w := httptest.NewRecorder()

	s.handleRefresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestHandleRefresh_InvalidToken(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		DatabasePath: ":memory:",
	}

	// Create server struct directly to avoid template loading
	s := &Server{
		db:     db,
		config: cfg,
	}

	// Set a dummy token for the test
	os.Setenv("REFRESH_TOKEN", "correct-token")
	defer os.Unsetenv("REFRESH_TOKEN")

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()

	s.handleRefresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestHandleRefresh_NoTokenConfigured(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	cfg := &config.Config{
		DatabasePath: ":memory:",
	}

	// Create server struct directly to avoid template loading
	s := &Server{
		db:     db,
		config: cfg,
	}

	// Ensure no token is set
	os.Unsetenv("REFRESH_TOKEN")

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	w := httptest.NewRecorder()

	s.handleRefresh(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestHandleRefresh_ValidToken(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create the required table structure
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
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	cfg := &config.Config{
		DatabasePath:   ":memory:",
		UploadDatabase: false, // Don't try to upload during tests
	}

	// Create server struct directly to avoid template loading
	s := &Server{
		db:     db,
		config: cfg,
	}

	// Set a test token
	os.Setenv("REFRESH_TOKEN", "test-token-123")
	defer os.Unsetenv("REFRESH_TOKEN")

	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req.Header.Set("Authorization", "Bearer test-token-123")
	w := httptest.NewRecorder()

	s.handleRefresh(w, req)

	// Should return 202 Accepted since it's async
	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status %d, got %d", http.StatusAccepted, w.Code)
	}

	// Parse response
	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "accepted" {
		t.Errorf("Expected status 'accepted', got '%s'", response["status"])
	}

	// Give the background goroutine a moment to start
	time.Sleep(100 * time.Millisecond)
}

func TestHandleRefresh_ConcurrentRequests(t *testing.T) {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Create the required table structure
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
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	cfg := &config.Config{
		DatabasePath:   ":memory:",
		UploadDatabase: false,
	}

	// Create server struct directly to avoid template loading
	s := &Server{
		db:     db,
		config: cfg,
	}

	os.Setenv("REFRESH_TOKEN", "test-token-123")
	defer os.Unsetenv("REFRESH_TOKEN")

	// First request - should be accepted
	req1 := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req1.Header.Set("Authorization", "Bearer test-token-123")
	w1 := httptest.NewRecorder()

	s.handleRefresh(w1, req1)

	if w1.Code != http.StatusAccepted {
		t.Errorf("First request: expected status %d, got %d", http.StatusAccepted, w1.Code)
	}

	// Second request immediately after - should be rejected
	req2 := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	req2.Header.Set("Authorization", "Bearer test-token-123")
	w2 := httptest.NewRecorder()

	s.handleRefresh(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request: expected status %d, got %d", http.StatusTooManyRequests, w2.Code)
	}

	// Parse response
	var response map[string]string
	if err := json.NewDecoder(w2.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "already_running" {
		t.Errorf("Expected status 'already_running', got '%s'", response["status"])
	}

	// Give the background goroutine time to complete
	time.Sleep(100 * time.Millisecond)
}
