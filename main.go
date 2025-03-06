package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/benfb/oaamonitor/config"
	"github.com/benfb/oaamonitor/database"
	"github.com/benfb/oaamonitor/models"
	"github.com/benfb/oaamonitor/refresher"
	"github.com/benfb/oaamonitor/server"
	"github.com/benfb/oaamonitor/storage"
	_ "github.com/mattn/go-sqlite3"
)

type Server struct {
	db     *sql.DB
	router *http.ServeMux
	tmpl   *template.Template
	config *config.Config
}

func NewServer(db *sql.DB, cfg *config.Config) (*Server, error) {
	s := &Server{
		db:     db,
		router: http.NewServeMux(),
		config: cfg,
	}

	// Parse all templates
	templates, err := filepath.Glob("templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to find templates: %v", err)
	}

	s.tmpl, err = template.ParseFiles(templates...)
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %v", err)
	}

	s.routes()
	return s, nil
}

func (s *Server) routes() {
	s.router.HandleFunc("/", s.handleIndexPage)
	s.router.HandleFunc("GET /player/{id}", s.handlePlayerPage)
	s.router.HandleFunc("GET /team/{id}", s.handleTeamPage)
	s.router.HandleFunc("GET /search", s.handleSearch)
	s.router.HandleFunc("GET /download", s.handleDatabaseDownload)
	s.router.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) renderTemplate(w http.ResponseWriter, tmplName string, data any) {
	// Set the content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Execute the template
	err := s.tmpl.ExecuteTemplate(w, tmplName, data)
	if err != nil {
		log.Printf("Error rendering template %s: %v", tmplName, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	var results []models.Player
	players, err := models.FetchPlayers(s.db)
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

func (s *Server) handlePlayerPage(w http.ResponseWriter, r *http.Request) {
	playerID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		log.Println(playerID)
		http.Error(w, "Invalid player ID", http.StatusBadRequest)
		return
	}

	// Get requested season or use current year as default
	seasonParam := r.URL.Query().Get("season")
	selectedSeason := time.Now().Year()
	if seasonParam != "" {
		if season, err := strconv.Atoi(seasonParam); err == nil {
			selectedSeason = season
		}
	}

	// Get available seasons
	seasons, err := models.FetchSeasons(s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Default to most recent season if no specific selection
	if len(seasons) > 0 && seasonParam == "" {
		selectedSeason = seasons[0]
	}

	playerStats, playerName, playerPosition, err := models.FetchPlayerStats(s.db, playerID, selectedSeason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	teams, err := models.FetchTeams(s.db)
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

	s.renderTemplate(w, "player.html", data)
}

func (s *Server) handleTeamPage(w http.ResponseWriter, r *http.Request) {
	teamName := strings.ToLower(r.PathValue("id"))
	teamName = normalizeTeamName(teamName)

	// Get requested season or use current year as default
	seasonParam := r.URL.Query().Get("season")
	selectedSeason := time.Now().Year()
	if seasonParam != "" {
		if season, err := strconv.Atoi(seasonParam); err == nil {
			selectedSeason = season
		}
	}

	// Get available seasons
	seasons, err := models.FetchSeasons(s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Default to most recent season if no specific selection
	if len(seasons) > 0 && seasonParam == "" {
		selectedSeason = seasons[0]
	}

	teamStats, capitalizedTeamName, err := models.FetchTeamStats(s.db, teamName, selectedSeason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	teams, err := models.FetchTeams(s.db)
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
	s.renderTemplate(w, "team.html", data)
}

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

func (s *Server) handleIndexPage(w http.ResponseWriter, r *http.Request) {
	players, err := models.FetchPlayers(s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	teams, err := models.FetchTeams(s.db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	playerDifferences, err := models.FetchPlayerDifferences(s.db, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sevenDayTrends, err := models.FetchNDayTrends(s.db, 7, 1)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	thirtyDayTrends, err := models.FetchNDayTrends(s.db, 30, 3)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dbSize, err := getDatabaseSize(s.config.DatabasePath)
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

	s.renderTemplate(w, "index.html", data)
}

func main() {
	// Load configuration
	cfg := config.NewConfig()

	// Download database if configured
	if cfg.DownloadDatabase {
		log.Println("Downloading database from Fly Storage")
		if err := storage.DownloadDatabase(cfg.DatabasePath); err != nil {
			log.Fatalf("Failed to download database: %v", err)
		}
	}

	// Initialize database connection
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create and initialize server
	srv, err := server.New(db, cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start HTTP server in a goroutine
	httpServer := &http.Server{
		Addr:         ":" + config.GetEnvValue("PORT", "8080"),
		Handler:      srv,
		ReadTimeout:  time.Duration(cfg.RequestTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.RequestTimeout) * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Println("Starting server on port", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// Start periodic refresh in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		refresher.RunPeriodically(ctx, cfg, time.Duration(cfg.RefreshRate)*time.Second, refresher.GetLatestOAA)
	}()

	// Wait for interrupt signal
	<-stop
	log.Println("Shutting down server...")

	// Cancel the refresh goroutine
	cancel()

	// Shutdown the HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	// Wait for goroutines to finish
	wg.Wait()
	log.Println("Server exited properly")
}

// normalizeTeamName normalizes team names to a standard format
func normalizeTeamName(teamName string) string {
	switch teamName {
	case "diamondbacks", "dbacks":
		return "d-backs"
	case "blue-jays", "red-sox", "white-sox":
		return strings.ReplaceAll(teamName, "-", " ")
	case "blue+jays", "red+sox", "white+sox":
		return strings.ReplaceAll(teamName, "+", " ")
	case "bluejays":
		return "blue jays"
	case "redsox":
		return "red sox"
	case "whitesox":
		return "white sox"
	default:
		return teamName
	}
}

func getDatabaseSize(dbPath string) (string, error) {
	fileInfo, err := os.Stat(dbPath)
	if err != nil {
		return "", err
	}
	size := fileInfo.Size()
	return fmt.Sprintf("%.2f MB", float64(size)/1024/1024), nil
}
