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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/benfb/oaamonitor/refresher"
	_ "github.com/mattn/go-sqlite3"
)

// Stat represents a row in the outs_above_average table
type Stat struct {
	PlayerID             int     `json:"player_id"`
	Name                 string  `json:"name"`
	Team                 string  `json:"team"`
	OAA                  int     `json:"oaa"`
	Date                 string  `json:"date"`
	ActualSuccessRate    float64 `json:"actual_success_rate"`
	EstimatedSuccessRate float64 `json:"estimated_success_rate"`
	DiffSuccessRate      float64 `json:"diff_success_rate"`
}

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

func downloadDatabaseFromStorage(dbPath string) error {
	sdkConfig, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Printf("Failed to load SDK configuration: %v", err)
		return err
	}
	// Create S3 service client
	svc := s3.NewFromConfig(sdkConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://fly.storage.tigris.dev")
		o.Region = "auto"
	})
	// Download the SQLite database file
	file, err := os.Create(dbPath)
	if err != nil {
		log.Printf("Failed to create database file: %v", err)
		return err
	}
	defer file.Close()
	resp, err := svc.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String("oaamonitor"),
		Key:    aws.String("oaamonitor.db"),
	})
	if err != nil {
		log.Printf("Failed to download database file: %v", err)
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		log.Printf("Failed to save database file: %v", err)
		return err
	}
	return nil
}

func main() {
	// Find the SQLite database
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./data/oaamonitor.db"
	}
	downloadDatabaseFromStorage(dbPath)
	runRepeatedly(refresher.GetLatestOAA, 1*time.Hour)
	// Open the SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		players, err := fetchPlayers(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		teams, err := fetchTeams(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tmplPath := filepath.Join("templates", "index.html")
		tmpl, err := template.New("index.html").Funcs(template.FuncMap{
			"json": func(v interface{}) (template.JS, error) {
				a, err := json.Marshal(v)
				if err != nil {
					return "", err
				}
				return template.JS(a), nil
			},
		}).ParseFiles(tmplPath, "templates/header.html", "templates/footer.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := struct {
			Title   string
			Players []Player
			Teams   []string
		}{
			Title:   "Baseball Stats",
			Players: players,
			Teams:   teams,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "header", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tmpl.ExecuteTemplate(w, "content", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tmpl.ExecuteTemplate(w, "footer", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Create an HTTP handler for player-specific pages
	http.HandleFunc("/player/", func(w http.ResponseWriter, r *http.Request) {
		playerIDStr := r.URL.Path[len("/player/"):]
		playerID, err := strconv.Atoi(playerIDStr)
		if err != nil {
			http.Error(w, "Invalid player ID", http.StatusBadRequest)
			return
		}

		playerStats, playerName, err := fetchPlayerStats(db, playerID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tmplPath := filepath.Join("templates", "player.html")
		tmpl, err := template.New("player.html").Funcs(template.FuncMap{
			"json": func(v interface{}) (template.JS, error) {
				a, err := json.Marshal(v)
				if err != nil {
					return "", err
				}
				return template.JS(a), nil
			},
		}).ParseFiles(tmplPath, "templates/header.html", "templates/footer.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := struct {
			Title       string
			PlayerName  string
			PlayerStats []Stat
		}{
			Title:       fmt.Sprintf("%s Outs Above Average", playerName),
			PlayerName:  playerName,
			PlayerStats: playerStats,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "header", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tmpl.ExecuteTemplate(w, "content", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tmpl.ExecuteTemplate(w, "footer", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Create an HTTP handler for team-specific pages
	http.HandleFunc("/team/", func(w http.ResponseWriter, r *http.Request) {
		teamName := strings.ToLower(r.URL.Path[len("/team/"):])
		teamName = normalizeTeamName(teamName)

		teamStats, capitalizedTeamName, err := fetchTeamStats(db, teamName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		tmplPath := filepath.Join("templates", "team.html")
		tmpl, err := template.New("team.html").Funcs(template.FuncMap{
			"json": func(v interface{}) (template.JS, error) {
				a, err := json.Marshal(v)
				if err != nil {
					return "", err
				}
				return template.JS(a), nil
			},
		}).ParseFiles(tmplPath, "templates/header.html", "templates/footer.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := struct {
			Title     string
			TeamName  string
			TeamStats []Stat
		}{
			Title:     fmt.Sprintf("%s Outs Above Average", capitalizedTeamName),
			TeamName:  capitalizedTeamName,
			TeamStats: teamStats,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, "header", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tmpl.ExecuteTemplate(w, "content", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := tmpl.ExecuteTemplate(w, "footer", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Serve static files (e.g., Chart.js)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Start the server
	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// fetchPlayers retrieves distinct player IDs and names from the database in alphabetical order
func fetchPlayers(db *sql.DB) ([]Player, error) {
	rows, err := db.Query("SELECT DISTINCT player_id, full_name FROM outs_above_average ORDER BY last_name, first_name DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var players []Player
	for rows.Next() {
		var playerID int
		var playerName string
		if err := rows.Scan(&playerID, &playerName); err != nil {
			return nil, err
		}
		player := Player{
			ID:   playerID,
			Name: playerName,
		}
		players = append(players, player)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return players, nil
}

// Player represents a player with their ID and name
type Player struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// fetchPlayerStats retrieves statistics for a specific player from the database
func fetchPlayerStats(db *sql.DB, playerID int) ([]Stat, string, error) {
	rows, err := db.Query(`WITH ranked_data AS (
		SELECT 
			player_id,
			full_name,
			team,
			oaa,
			DATE(loaded_at) AS date,
			actual_success_rate,
			estimated_success_rate,
			diff_success_rate,
			ROW_NUMBER() OVER (
				PARTITION BY player_id, DATE(loaded_at)
				ORDER BY loaded_at DESC
			) AS rn
		FROM outs_above_average
	)
	SELECT 
		player_id,
		full_name,
		team,
		oaa,
		date,
		actual_success_rate,
		estimated_success_rate,
		diff_success_rate
	FROM ranked_data
	WHERE rn = 1
	ORDER BY player_id, date;"`, playerID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var stats []Stat
	var playerName string
	for rows.Next() {
		var stat Stat
		if err := rows.Scan(&stat.PlayerID, &stat.Name, &stat.Team, &stat.OAA, &stat.Date, &stat.ActualSuccessRate, &stat.EstimatedSuccessRate, &stat.DiffSuccessRate); err != nil {
			return nil, "", err
		}
		stats = append(stats, stat)
		playerName = stat.Name
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	return stats, playerName, nil
}

// fetchTeamStats retrieves statistics for all players in a specific team from the database
func fetchTeamStats(db *sql.DB, teamName string) ([]Stat, string, error) {
	rows, err := db.Query("SELECT player_id, full_name, team, oaa, date(loaded_at) as date, actual_success_rate, estimated_success_rate, diff_success_rate FROM outs_above_average WHERE (team) IN (SELECT team FROM outs_above_average WHERE LOWER(team) = ? GROUP BY team)", teamName)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var stats []Stat
	var capitalizedTeamName string
	for rows.Next() {
		var stat Stat
		if err := rows.Scan(&stat.PlayerID, &stat.Name, &stat.Team, &stat.OAA, &stat.Date, &stat.ActualSuccessRate, &stat.EstimatedSuccessRate, &stat.DiffSuccessRate); err != nil {
			return nil, "", err
		}
		stats = append(stats, stat)
		capitalizedTeamName = stat.Team
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	return stats, capitalizedTeamName, nil
}

// fetchTeams retrieves distinct team names from the database
func fetchTeams(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT DISTINCT team FROM outs_above_average ORDER BY team")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []string
	for rows.Next() {
		var team string
		if err := rows.Scan(&team); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return teams, nil
}

// normalizeTeamName normalizes team names to a standard format
func normalizeTeamName(teamName string) string {
	switch teamName {
	case "diamondbacks", "dbacks":
		return "d-backs"
	case "white-sox", "whitesox", "white+sox":
		return "white sox"
	case "red-sox", "redsox", "red+sox":
		return "red sox"
	default:
		return teamName
	}
}
