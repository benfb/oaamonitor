package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/benfb/oaamonitor/config"
	"github.com/benfb/oaamonitor/models"
	"github.com/benfb/oaamonitor/refresher"
	"github.com/benfb/oaamonitor/storage"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var db *sql.DB

func runRepeatedly(fn func(), interval time.Duration) {
	go func() {
		// fn() // Run once before timer takes effect
		next10AM := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), time.Now().UTC().Day(), 10, 0, 0, 0, time.UTC)
		if next10AM.Before(time.Now().UTC()) {
			next10AM = next10AM.AddDate(0, 0, 1)
		}
		durationUntilNext10AM := next10AM.Sub(time.Now().UTC())
		fmt.Println("Sleeping for ", durationUntilNext10AM)
		time.Sleep(durationUntilNext10AM)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			log.Println("Refreshing data...")
			fn()
		}
	}()
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

	data := struct {
		Title       string
		PlayerName  string
		PlayerStats []models.Stat
	}{
		Title:       fmt.Sprintf("%s Outs Above Average", playerName),
		PlayerName:  playerName,
		PlayerStats: playerStats,
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

	data := struct {
		Title     string
		TeamName  string
		TeamStats []models.Stat
	}{
		Title:     fmt.Sprintf("%s Outs Above Average", capitalizedTeamName),
		TeamName:  capitalizedTeamName,
		TeamStats: teamStats,
	}
	if err := renderTemplate(w, "team.html", data); err != nil {
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

	data := struct {
		Title   string
		Players []models.Player
		Teams   []string
	}{
		Title:   "Outs Above Average Monitor",
		Players: players,
		Teams:   teams,
	}

	if err := renderTemplate(w, "index.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func main() {
	cfg := config.NewConfig()
	if cfg.DownloadDatabase {
		storage.DownloadDatabase(cfg.DatabasePath)
	}
	runRepeatedly(refresher.GetLatestOAA, time.Duration(cfg.RefreshRate)*time.Second)
	// Open the SQLite database
	db = cfg.DB
	defer db.Close()

	handler := NewServer(log.New(os.Stdout, "oaamonitor: ", log.LstdFlags))

	// Start the server
	log.Println("Starting server on :8080")
	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func NewServer(logger *log.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndexPage)
	mux.HandleFunc("GET /player/{id}", handlePlayerPage)
	mux.HandleFunc("GET /team/{id}", handleTeamPage)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	return mux
}

func renderTemplate(w http.ResponseWriter, templatePath string, data interface{}) error {
	tmplPath := filepath.Join("templates", templatePath)
	tmpl, err := template.New(tmplPath).Funcs(template.FuncMap{
		"json": func(v interface{}) (template.JS, error) {
			a, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return template.JS(a), nil
		},
		"Title": func(s string) string {
			return cases.Title(language.English, cases.NoLower).String(s)
		},
	}).ParseFiles(tmplPath, "templates/header.html", "templates/footer.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.ExecuteTemplate(w, "header", data)
	tmpl.ExecuteTemplate(w, "content", data)
	tmpl.ExecuteTemplate(w, "footer", data)
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
