package models

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Player represents a player with their ID and name
type Player struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Stat represents a row in the outs_above_average table
type Stat struct {
	PlayerID             int       `json:"player_id"`
	Name                 string    `json:"name"`
	Team                 string    `json:"team"`
	Position             string    `json:"position"`
	OAA                  int       `json:"oaa"`
	Date                 time.Time `json:"date"`
	ActualSuccessRate    float64   `json:"actual_success_rate"`
	EstimatedSuccessRate float64   `json:"estimated_success_rate"`
	DiffSuccessRate      float64   `json:"diff_success_rate"`
}

// PlayerDifference represents a player's OAA trend over a time period
type PlayerDifference struct {
	PlayerID    int    `json:"player_id"`
	Name        string `json:"name"`
	Team        string `json:"team"`
	Position    string `json:"position"`
	CurrentOAA  int    `json:"current_oaa"`
	PreviousOAA int    `json:"previous_oaa"`
	Difference  int    `json:"difference"`
}

func FetchTeamStats(db *sql.DB, teamName string, season int) ([]Stat, string, error) {
	rows, err := db.Query(`
	WITH latest_positions AS (
		SELECT player_id, primary_position,
			ROW_NUMBER() OVER (PARTITION BY player_id ORDER BY date DESC) as rn
		FROM outs_above_average
		WHERE LOWER(team) = ? AND primary_position IS NOT NULL AND strftime('%Y', date) = ?
	)
	SELECT
		o.player_id,
		o.full_name,
		o.team,
		COALESCE(lp.primary_position, 'N/A') as position,
		o.oaa,
		o.date,
		o.actual_success_rate,
		o.estimated_success_rate,
		o.diff_success_rate
	FROM outs_above_average o
	LEFT JOIN latest_positions lp ON o.player_id = lp.player_id AND lp.rn = 1
	WHERE LOWER(o.team) = ? AND strftime('%Y', o.date) = ?
	GROUP BY o.player_id, o.date
	ORDER BY o.last_name, o.date;`, teamName, strconv.Itoa(season), teamName, strconv.Itoa(season))
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var stats []Stat
	var capitalizedTeamName string
	for rows.Next() {
		var stat Stat
		if err := rows.Scan(&stat.PlayerID, &stat.Name, &stat.Team, &stat.Position, &stat.OAA, &stat.Date, &stat.ActualSuccessRate, &stat.EstimatedSuccessRate, &stat.DiffSuccessRate); err != nil {
			return nil, "", err
		}
		stats = append(stats, stat)
		capitalizedTeamName = stat.Team
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	// If no data found for the specified season, try to get team name from any season
	if len(stats) == 0 {
		var teamNameResult string
		err := db.QueryRow(`
			SELECT team FROM outs_above_average 
			WHERE LOWER(team) = ? 
			LIMIT 1
		`, teamName).Scan(&teamNameResult)
		if err == nil {
			capitalizedTeamName = teamNameResult
		}
	}

	return stats, capitalizedTeamName, nil
}

type SparklinePoint struct {
	Date time.Time
	OAA  int
}

type PlayerStats struct {
	PlayerID   int
	Name       string
	Position   string
	LatestOAA  int
	OAAHistory []SparklinePoint
}

// MapStatsByPlayerID returns a slice of PlayerStats sorted by current OAA in descending order
func MapStatsByPlayerID(stats []Stat) []PlayerStats {
	statsMap := make(map[int]*PlayerStats)

	for _, stat := range stats {
		if entry, ok := statsMap[stat.PlayerID]; !ok {
			statsMap[stat.PlayerID] = &PlayerStats{
				PlayerID:   stat.PlayerID,
				Name:       stat.Name,
				Position:   stat.Position,
				LatestOAA:  stat.OAA,
				OAAHistory: []SparklinePoint{{Date: stat.Date, OAA: stat.OAA}},
			}
		} else {
			// Check if current stat's date is more recent than our stored latest date
			latestDate := time.Time{}
			if len(entry.OAAHistory) > 0 {
				// Find the most recent date in history
				for _, point := range entry.OAAHistory {
					if point.Date.After(latestDate) {
						latestDate = point.Date
					}
				}
			}

			// Update latest OAA if this is a more recent stat
			if stat.Date.After(latestDate) {
				entry.LatestOAA = stat.OAA
			}

			// Add to history
			entry.OAAHistory = append(entry.OAAHistory, SparklinePoint{Date: stat.Date, OAA: stat.OAA})

			// Use non-N/A position if available
			if stat.Position != "N/A" {
				entry.Position = stat.Position
			}
		}
	}

	// Convert map to slice
	playerStatsList := make([]PlayerStats, 0, len(statsMap))
	for _, v := range statsMap {
		playerStatsList = append(playerStatsList, *v)
	}

	// Sort the slice by LatestOAA in descending order
	sort.Slice(playerStatsList, func(i, j int) bool {
		return playerStatsList[i].LatestOAA > playerStatsList[j].LatestOAA
	})

	return playerStatsList
}

// FetchPlayers retrieves distinct player IDs and names from the database in alphabetical order
func FetchPlayers(db *sql.DB) ([]Player, error) {
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

// FetchPlayerStats retrieves statistics for a specific player from the database
func FetchPlayerStats(db *sql.DB, playerID int, season int) ([]Stat, string, string, error) {
	rows, err := db.Query(`
	SELECT
		player_id,
		full_name,
		team,
		COALESCE(primary_position, 'N/A') as position,
		oaa,
		date,
		actual_success_rate,
		estimated_success_rate,
		diff_success_rate
	FROM outs_above_average
	WHERE player_id = ? AND strftime('%Y', date) = ?
	ORDER BY player_id, date;`, playerID, strconv.Itoa(season))
	if err != nil {
		return nil, "", "", err
	}
	defer rows.Close()

	var stats []Stat
	var playerName string
	var playerPosition string
	for rows.Next() {
		var stat Stat
		if err := rows.Scan(&stat.PlayerID, &stat.Name, &stat.Team, &stat.Position, &stat.OAA, &stat.Date, &stat.ActualSuccessRate, &stat.EstimatedSuccessRate, &stat.DiffSuccessRate); err != nil {
			return nil, "", "", err
		}
		stats = append(stats, stat)
		playerName = stat.Name
		if stat.Position != "N/A" {
			playerPosition = stat.Position
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", "", err
	}

	// If no data found for the specified season, return empty slice but not an error
	if len(stats) == 0 {
		// Try to get player name and position from any season
		var name, position string
		err := db.QueryRow(`
			SELECT full_name, COALESCE(primary_position, 'N/A')
			FROM outs_above_average
			WHERE player_id = ?
			ORDER BY date DESC LIMIT 1
		`, playerID).Scan(&name, &position)
		if err != nil {
			return []Stat{}, "", "N/A", nil
		}
		return []Stat{}, name, position, nil
	}

	// If no valid position was found, set it to "N/A"
	if playerPosition == "" {
		playerPosition = "N/A"
	}

	return stats, playerName, playerPosition, nil
}

// FetchTeams retrieves distinct team names from the database
func FetchTeams(db *sql.DB) ([]string, error) {
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

// GetTeamAbbreviation returns the MLB abbreviation for a given team name
func GetTeamAbbreviation(teamName string) string {
	abbreviations := map[string]string{
		"Angels":    "LAA",
		"Astros":    "HOU",
		"Athletics": "OAK",
		"Blue Jays": "TOR",
		"Braves":    "ATL",
		"Brewers":   "MIL",
		"Cardinals": "STL",
		"Cubs":      "CHC",
		"D-backs":   "ARI",
		"Dodgers":   "LAD",
		"Giants":    "SF",
		"Guardians": "CLE",
		"Mariners":  "SEA",
		"Marlins":   "MIA",
		"Mets":      "NYM",
		"Nationals": "WSH",
		"Orioles":   "BAL",
		"Padres":    "SD",
		"Phillies":  "PHI",
		"Pirates":   "PIT",
		"Rangers":   "TEX",
		"Rays":      "TB",
		"Red Sox":   "BOS",
		"Reds":      "CIN",
		"Rockies":   "COL",
		"Royals":    "KC",
		"Tigers":    "DET",
		"Twins":     "MIN",
		"White Sox": "CWS",
		"Yankees":   "NYY",
	}

	if abbr, ok := abbreviations[teamName]; ok {
		return abbr
	}
	return ""
}

// FetchPlayerDifferences retrieves the players with the biggest differences between the current and previous oaa totals from the database
func FetchPlayerDifferences(db *sql.DB, limit int) ([]PlayerDifference, error) {
	// Check if there are any differences for any player between the current and previous OAA totals
	var count int
	if err := db.QueryRow(`
	WITH current AS (
		SELECT player_id, oaa FROM outs_above_average WHERE date = (SELECT MAX(date) FROM outs_above_average)
	),
	previous AS (
		SELECT player_id, oaa FROM outs_above_average WHERE date = (SELECT MAX(date) FROM outs_above_average WHERE date < (SELECT MAX(date) FROM outs_above_average))
	)
	SELECT COUNT(*) FROM current JOIN previous ON current.player_id = previous.player_id WHERE current.oaa != previous.oaa;
	`).Scan(&count); err != nil {
		return nil, err
	}
	var rows *sql.Rows
	var err error

	// If there are no differences, fetch the differences between the current and previous OAA totals
	if count == 0 {
		rows, err = db.Query(`
			SELECT
				current.player_id,
				current.full_name,
				current.team,
				COALESCE(current.primary_position, 'N/A') AS position,
				current.oaa AS current_oaa,
				previous.oaa AS previous_oaa,
				current.oaa - previous.oaa AS difference
			FROM outs_above_average AS current
			JOIN outs_above_average AS previous
			ON current.player_id = previous.player_id
			WHERE current.date = date((SELECT MAX(date) FROM outs_above_average), '-1 day')
			AND previous.date = date((SELECT MAX(date) FROM outs_above_average WHERE date < (SELECT MAX(date) FROM outs_above_average)), '-1 day')
			AND current.oaa != previous.oaa
			ORDER BY difference DESC
			LIMIT ?;
			`, limit)
	} else {
		// Otherwise, fetch the differences between the current and previous OAA totals starting from the previous day
		rows, err = db.Query(`
			SELECT
				current.player_id,
				current.full_name,
				current.team,
				COALESCE(current.primary_position, 'N/A') AS position,
				current.oaa AS current_oaa,
				previous.oaa AS previous_oaa,
				current.oaa - previous.oaa AS difference
			FROM outs_above_average AS current
			JOIN outs_above_average AS previous
			ON current.player_id = previous.player_id
			WHERE current.date = (SELECT MAX(date) FROM outs_above_average)
			AND previous.date = (SELECT MAX(date) FROM outs_above_average WHERE date < (SELECT MAX(date) FROM outs_above_average))
			AND current.oaa != previous.oaa
			ORDER BY difference DESC
			LIMIT ?;`, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var differences []PlayerDifference
	for rows.Next() {
		var difference PlayerDifference
		if err := rows.Scan(&difference.PlayerID, &difference.Name, &difference.Team, &difference.Position, &difference.CurrentOAA, &difference.PreviousOAA, &difference.Difference); err != nil {
			return nil, err
		}
		differences = append(differences, difference)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return differences, nil
}

func FetchNDayTrends(db *sql.DB, daysAgo, threshold int) ([]PlayerDifference, error) {
	query := fmt.Sprintf(`
	WITH latest_positions AS (
		SELECT player_id, primary_position,
			ROW_NUMBER() OVER (PARTITION BY player_id ORDER BY date DESC) as rn
		FROM outs_above_average
		WHERE primary_position IS NOT NULL
	),
	recent_data AS (
		SELECT
			o.player_id,
			o.full_name,
			o.team,
			COALESCE(lp.primary_position, 'N/A') as position,
			o.oaa,
			o.date
		FROM outs_above_average o
		LEFT JOIN latest_positions lp ON o.player_id = lp.player_id AND lp.rn = 1
		WHERE o.date >= (SELECT DATE(MAX(date), '-%d days') FROM outs_above_average)
	),
	player_trends AS (
		SELECT
			player_id,
			full_name,
			team,
			position,
			FIRST_VALUE(oaa) OVER (PARTITION BY player_id ORDER BY date ASC) as start_oaa,
			FIRST_VALUE(oaa) OVER (PARTITION BY player_id ORDER BY date DESC) as end_oaa,
			ROW_NUMBER() OVER (PARTITION BY player_id) as rn
		FROM recent_data
	)
	SELECT
		player_id,
		full_name,
		team,
		position,
		start_oaa,
		end_oaa,
		(end_oaa - start_oaa) as difference
	FROM player_trends
	WHERE rn = 1 AND ABS(end_oaa - start_oaa) > %d
	ORDER BY difference DESC
	LIMIT 100;
	`, daysAgo, threshold)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trends []PlayerDifference
	for rows.Next() {
		var trend PlayerDifference
		if err := rows.Scan(&trend.PlayerID, &trend.Name, &trend.Team, &trend.Position, &trend.PreviousOAA, &trend.CurrentOAA, &trend.Difference); err != nil {
			return nil, err
		}
		trends = append(trends, trend)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return trends, nil
}

// FetchSeasons retrieves distinct seasons (years) from the database
func FetchSeasons(db *sql.DB) ([]int, error) {
	rows, err := db.Query("SELECT DISTINCT strftime('%Y', date) as year FROM outs_above_average ORDER BY year DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seasons []int
	for rows.Next() {
		var yearStr string
		if err := rows.Scan(&yearStr); err != nil {
			return nil, err
		}

		year, err := strconv.Atoi(yearStr)
		if err != nil {
			return nil, err
		}

		seasons = append(seasons, year)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return seasons, nil
}

// FetchPlayerSeasons returns the seasons in which the player has recorded stats
func FetchPlayerSeasons(db *sql.DB, playerID int) ([]int, error) {
	rows, err := db.Query(`
		SELECT DISTINCT strftime('%Y', date) as year
		FROM outs_above_average
		WHERE player_id = ?
		ORDER BY year DESC
	`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seasons []int
	for rows.Next() {
		var yearStr string
		if err := rows.Scan(&yearStr); err != nil {
			return nil, err
		}
		year, err := strconv.Atoi(yearStr)
		if err != nil {
			return nil, err
		}
		seasons = append(seasons, year)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return seasons, nil
}

// FetchTeamSeasons returns the seasons in which the team has recorded stats
func FetchTeamSeasons(db *sql.DB, teamName string) ([]int, error) {
	rows, err := db.Query(`
		SELECT DISTINCT strftime('%Y', date) as year
		FROM outs_above_average
		WHERE LOWER(team) = ?
		ORDER BY year DESC
	`, strings.ToLower(teamName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seasons []int
	for rows.Next() {
		var yearStr string
		if err := rows.Scan(&yearStr); err != nil {
			return nil, err
		}
		year, err := strconv.Atoi(yearStr)
		if err != nil {
			return nil, err
		}
		seasons = append(seasons, year)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return seasons, nil
}
