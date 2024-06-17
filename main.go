package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/benfb/oaamonitor/config"
	"github.com/benfb/oaamonitor/models"
	"github.com/benfb/oaamonitor/refresher"
	"github.com/benfb/oaamonitor/storage"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func runRepeatedly(fn func(), interval time.Duration) {
	go func() {
		// fn() // Run once before timer takes effect
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			log.Println("Refreshing data...")
			fn()
		}
	}()
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	var results []models.Player
	players, err := models.FetchPlayers(db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, player := range players {
		if strings.Contains(strings.ToLower(player.Name), strings.ToLower(query)) {
			results = append(results, player)
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
	})
}

func handlePlayerPage(w http.ResponseWriter, r *http.Request) {
	playerID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		log.Println(playerID)
		http.Error(w, "Invalid player ID", http.StatusBadRequest)
		return
	}

	playerStats, playerName, err := models.FetchPlayerStats(db, playerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	teams, err := models.FetchTeams(db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Title       string
		PlayerName  string
		PlayerStats []models.Stat
		Teams       []string
	}{
		Title:       fmt.Sprintf("%s Outs Above Average", playerName),
		PlayerName:  playerName,
		PlayerStats: playerStats,
		Teams:       teams,
	}

	if err := renderTemplate(w, "player.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func handleTeamPage(w http.ResponseWriter, r *http.Request) {
	teamName := strings.ToLower(r.PathValue("id"))
	teamName = normalizeTeamName(teamName)
	teamStats, capitalizedTeamName, err := models.FetchTeamStats(db, teamName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	teams, err := models.FetchTeams(db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	playerStats := models.MapStatsByPlayerID(teamStats)

	data := struct {
		Title          string
		TeamName       string
		TeamStats      []models.Stat
		Teams          []string
		SparklinesData map[int]struct {
			Name       string
			LatestOAA  int
			OAAHistory []models.SparklinePoint
		}
	}{
		Title:          fmt.Sprintf("%s Outs Above Average", capitalizedTeamName),
		TeamName:       capitalizedTeamName,
		TeamStats:      teamStats,
		Teams:          teams,
		SparklinesData: playerStats,
	}
	if err := renderTemplate(w, "team.html", data); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func handleIndexPage(w http.ResponseWriter, r *http.Request) {
	players, err := models.FetchPlayers(db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	teams, err := models.FetchTeams(db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	playerDifferences, err := models.FetchPlayerDifferences(db, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Title             string
		Players           []models.Player
		Teams             []string
		PlayerDifferences []models.PlayerDifference
	}{
		Title:             "Outs Above Average Monitor",
		Players:           players,
		Teams:             teams,
		PlayerDifferences: playerDifferences,
	}

	if err := renderTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func main() {
	cfg := config.NewConfig()
	if cfg.DownloadDatabase {
		log.Println("Downloading database from Fly Storage")
		storage.DownloadDatabase(cfg.DatabasePath)
	}
	runRepeatedly(refresher.GetLatestOAA, time.Duration(cfg.RefreshRate)*time.Second)
	// Open the SQLite database
	var err error
	db, err = sql.Open("sqlite3", cfg.DatabasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndexPage)
	mux.HandleFunc("GET /player/{id}", handlePlayerPage)
	mux.HandleFunc("GET /team/{id}", handleTeamPage)
	mux.HandleFunc("GET /search", handleSearch)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Start the server
	log.Println("Starting server on :8080")
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func renderTemplate(w http.ResponseWriter, templatePath string, data interface{}) error {
	tmplPath := filepath.Join("templates", templatePath)
	tmpl, err := template.New(tmplPath).ParseFiles(tmplPath, "templates/header.html", "templates/footer.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = tmpl.ExecuteTemplate(w, "header", data)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	err = tmpl.ExecuteTemplate(w, "content", data)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	err = tmpl.ExecuteTemplate(w, "footer", data)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return nil
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
