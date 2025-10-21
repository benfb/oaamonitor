package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/benfb/oaamonitor/database"
	"github.com/benfb/oaamonitor/models"
	"github.com/benfb/oaamonitor/refresher"
)

// parseSeasonParam extracts and parses the season query parameter, returning the selected season
// and available seasons. If no season is specified, it defaults to the most recent season.
func (s *Server) parseSeasonParam(r *http.Request) (selectedSeason int, seasons []int, err error) {
	seasonParam := r.URL.Query().Get("season")
	selectedSeason = time.Now().Year()

	if seasonParam != "" {
		if season, parseErr := strconv.Atoi(seasonParam); parseErr == nil {
			selectedSeason = season
		}
	}

	seasons, err = models.FetchSeasons(s.db.DB)
	if err != nil {
		return 0, nil, err
	}

	// Default to most recent season if no specific selection
	if len(seasons) > 0 && seasonParam == "" {
		selectedSeason = seasons[0]
	}

	return selectedSeason, seasons, nil
}

// handleIndexPage handles requests to the home page
func (s *Server) handleIndexPage(w http.ResponseWriter, r *http.Request) {
	players, err := models.FetchPlayers(s.db.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	teams, err := models.FetchTeams(s.db.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	playerDifferences, err := models.FetchPlayerDifferences(s.db.DB, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sevenDayTrends, err := models.FetchNDayTrends(s.db.DB, 7, 1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	thirtyDayTrends, err := models.FetchNDayTrends(s.db.DB, 30, 3)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dbSize, err := database.GetDatabaseSize(s.config.DatabasePath)
	if err != nil {
		dbSize = "Unknown"
	}

	data := struct {
		Title             string
		Players           []models.Player
		Teams             []string
		PlayerDifferences []models.PlayerDifference
		SevenDayTrends    []models.PlayerDifference
		ThirtyDayTrends   []models.PlayerDifference
		DatabaseSize      string
	}{
		Title:             "Outs Above Average Monitor",
		Players:           players,
		Teams:             teams,
		PlayerDifferences: playerDifferences,
		SevenDayTrends:    sevenDayTrends,
		ThirtyDayTrends:   thirtyDayTrends,
		DatabaseSize:      dbSize,
	}

	s.renderer.Render(w, "index.html", data)
}

// handlePlayerPage handles requests to the player page
func (s *Server) handlePlayerPage(w http.ResponseWriter, r *http.Request) {
	playerID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		log.Println(playerID)
		http.Error(w, "Invalid player ID", http.StatusBadRequest)
		return
	}

	selectedSeason, seasons, err := s.parseSeasonParam(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	playerStats, playerName, playerPosition, err := models.FetchPlayerStats(s.db.DB, playerID, selectedSeason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	teams, err := models.FetchTeams(s.db.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	title := playerName + " Outs Above Average"
	if playerPosition != "N/A" {
		title = fmt.Sprintf("%s (%s) Outs Above Average", playerName, playerPosition)
	}

	data := struct {
		Title          string
		PlayerID       int
		PlayerName     string
		PlayerPosition string
		PlayerStats    []models.Stat
		Teams          []string
		Seasons        []int
		SelectedSeason int
	}{
		Title:          title,
		PlayerID:       playerID,
		PlayerName:     playerName,
		PlayerPosition: playerPosition,
		PlayerStats:    playerStats,
		Teams:          teams,
		Seasons:        seasons,
		SelectedSeason: selectedSeason,
	}

	s.renderer.Render(w, "player.html", data)
}

// handleTeamPage handles requests to the team page
func (s *Server) handleTeamPage(w http.ResponseWriter, r *http.Request) {
	teamName := strings.ToLower(r.PathValue("id"))
	teamName = NormalizeTeamName(teamName)

	selectedSeason, seasons, err := s.parseSeasonParam(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	teamStats, capitalizedTeamName, err := models.FetchTeamStats(s.db.DB, teamName, selectedSeason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	teams, err := models.FetchTeams(s.db.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	playerStats := models.MapStatsByPlayerID(teamStats)

	data := struct {
		Title            string
		TeamName         string
		TeamStats        []models.Stat
		Teams            []string
		SparklinesData   []models.PlayerStats
		CurrentYear      string
		TeamAbbreviation string
		Seasons          []int
		SelectedSeason   int
	}{
		Title:            fmt.Sprintf("%s Outs Above Average", capitalizedTeamName),
		TeamName:         capitalizedTeamName,
		TeamStats:        teamStats,
		Teams:            teams,
		SparklinesData:   playerStats,
		CurrentYear:      strconv.Itoa(selectedSeason),
		TeamAbbreviation: models.GetTeamAbbreviation(capitalizedTeamName),
		Seasons:          seasons,
		SelectedSeason:   selectedSeason,
	}
	s.renderer.Render(w, "team.html", data)
}

// handleSearch handles player search requests
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	var results []models.Player
	players, err := models.FetchPlayers(s.db.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, player := range players {
		if strings.Contains(strings.ToLower(player.Name), strings.ToLower(query)) {
			results = append(results, player)
		}
	}
	json.NewEncoder(w).Encode(map[string]any{
		"results": results,
	})
}

// handleDatabaseDownload handles requests to download the database
func (s *Server) handleDatabaseDownload(w http.ResponseWriter, r *http.Request) {
	dbPath := s.config.DatabasePath
	dbFile, err := os.Open(dbPath)
	if err != nil {
		http.Error(w, "Unable to open database file", http.StatusInternalServerError)
		return
	}
	defer dbFile.Close()

	w.Header().Set("Content-Disposition", "attachment; filename=oaamonitor.db")
	w.Header().Set("Content-Type", "application/octet-stream")

	_, err = io.Copy(w, dbFile)
	if err != nil {
		http.Error(w, "Error occurred during file download", http.StatusInternalServerError)
		return
	}
}

// handleRefresh handles requests to refresh the OAA data from Baseball Savant
// The refresh runs asynchronously and returns immediately with 202 Accepted
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	// Check for bearer token authentication
	authHeader := r.Header.Get("Authorization")
	expectedToken := os.Getenv("REFRESH_TOKEN")

	if expectedToken == "" {
		log.Println("REFRESH_TOKEN not configured")
		http.Error(w, "Refresh endpoint not configured", http.StatusInternalServerError)
		return
	}

	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	// Use constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check if a refresh is already in progress
	s.refreshMutex.Lock()
	if s.refreshing {
		lastRefresh := s.lastRefreshAt.Format(time.RFC3339)
		s.refreshMutex.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{
			"status":         "already_running",
			"message":        "Refresh already in progress",
			"last_refresh":   lastRefresh,
		})
		return
	}
	s.refreshing = true
	s.refreshMutex.Unlock()

	// Start refresh asynchronously
	go func() {
		defer func() {
			s.refreshMutex.Lock()
			s.refreshing = false
			s.lastRefreshAt = time.Now()
			s.refreshMutex.Unlock()
		}()

		// Create a context with timeout for the refresh operation
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		log.Println("Background refresh triggered via API")
		if err := refresher.GetLatestOAA(ctx, s.config); err != nil {
			log.Printf("Error during background refresh: %v", err)
			return
		}
		log.Println("Background refresh completed successfully")
	}()

	// Return immediately with 202 Accepted
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "accepted",
		"message": "Refresh started in background",
	})
}
