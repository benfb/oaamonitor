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

func main() {
	// Find the SQLite database
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./data/oaamonitor.db"
	}
	// Open the SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create an HTTP handler for the root path
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
			Title:       fmt.Sprintf("Player Stats - %s", playerName),
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
			Title:     fmt.Sprintf("Team Stats - %s", capitalizedTeamName),
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

// fetchStats retrieves all statistics from the database
// func fetchStats(db *sql.DB) ([]Stat, error) {
// 	rows, err := db.Query("SELECT DISTINCT name player_id, name, team, oaa, date(loaded_at) as date, actual_success_rate, estimated_success_rate, diff_success_rate FROM outs_above_average WHERE (player_id, loaded_at) IN (SELECT player_id, MAX(loaded_at) FROM outs_above_average GROUP BY player_id)")
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	var stats []Stat
// 	for rows.Next() {
// 		var stat Stat
// 		if err := rows.Scan(&stat.PlayerID, &stat.Name, &stat.Team, &stat.OAA, &stat.Date, &stat.ActualSuccessRate, &stat.EstimatedSuccessRate, &stat.DiffSuccessRate); err != nil {
// 			return nil, err
// 		}
// 		stats = append(stats, stat)
// 	}
// 	if err := rows.Err(); err != nil {
// 		return nil, err
// 	}

// 	return stats, nil
// }

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
	rows, err := db.Query("SELECT player_id, full_name, team, oaa, date(loaded_at) as date, actual_success_rate, estimated_success_rate, diff_success_rate FROM outs_above_average WHERE (player_id, loaded_at) IN (SELECT player_id, MAX(loaded_at) FROM outs_above_average WHERE player_id = ? GROUP BY player_id)", playerID)
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
	rows, err := db.Query("SELECT player_id, full_name, team, oaa, date(loaded_at) as date, actual_success_rate, estimated_success_rate, diff_success_rate FROM outs_above_average WHERE (team, loaded_at) IN (SELECT team, MAX(loaded_at) FROM outs_above_average WHERE LOWER(team) = ? GROUP BY team)", teamName)
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
