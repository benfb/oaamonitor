package models

import (
	"database/sql"
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
	OAA                  int       `json:"oaa"`
	Date                 time.Time `json:"date"`
	ActualSuccessRate    float64   `json:"actual_success_rate"`
	EstimatedSuccessRate float64   `json:"estimated_success_rate"`
	DiffSuccessRate      float64   `json:"diff_success_rate"`
}

type PlayerDifference struct {
	PlayerID    int    `json:"player_id"`
	Name        string `json:"name"`
	Team        string `json:"team"`
	CurrentOAA  int    `json:"current_oaa"`
	PreviousOAA int    `json:"previous_oaa"`
	Difference  int    `json:"difference"`
}

// FetchTeamStats retrieves statistics for all players in a specific team from the database
func FetchTeamStats(db *sql.DB, teamName string) ([]Stat, string, error) {
	rows, err := db.Query(`
	SELECT 
		player_id,
		full_name,
		team,
		oaa,
		date,
		actual_success_rate,
		estimated_success_rate,
		diff_success_rate
	FROM outs_above_average
	WHERE LOWER(team) = ?
	GROUP BY player_id, date
	ORDER BY player_id, date;`, teamName)
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

type SparklinePoint struct {
	Date time.Time
	OAA  int
}

// Function which retruns a map of player ID to a struct containing the player's name, the latest OAA value, and a slice of OAA values ordered by date
func MapStatsByPlayerID(stats []Stat) map[int]struct {
	Name       string
	LatestOAA  int
	OAAHistory []SparklinePoint
} {
	statsMap := make(map[int]struct {
		Name       string
		LatestOAA  int
		OAAHistory []SparklinePoint
	})
	for _, stat := range stats {
		if _, ok := statsMap[stat.PlayerID]; !ok {
			statsMap[stat.PlayerID] = struct {
				Name       string
				LatestOAA  int
				OAAHistory []SparklinePoint
			}{
				Name:      stat.Name,
				LatestOAA: stat.OAA,
				OAAHistory: []SparklinePoint{{
					Date: stat.Date,
					OAA:  stat.OAA,
				}},
			}
		} else {
			statsMap[stat.PlayerID] = struct {
				Name       string
				LatestOAA  int
				OAAHistory []SparklinePoint
			}{
				Name:       statsMap[stat.PlayerID].Name,
				LatestOAA:  stat.OAA,
				OAAHistory: append(statsMap[stat.PlayerID].OAAHistory, SparklinePoint{stat.Date, stat.OAA}),
			}
		}
	}
	return statsMap
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
func FetchPlayerStats(db *sql.DB, playerID int) ([]Stat, string, error) {
	rows, err := db.Query(`
	SELECT 
		player_id,
		full_name,
		team,
		oaa,
		date,
		actual_success_rate,
		estimated_success_rate,
		diff_success_rate
	FROM outs_above_average
	WHERE player_id = ?
	ORDER BY player_id, date;`, playerID)
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
			current.oaa AS current_oaa,
			previous.oaa AS previous_oaa,
			current.oaa - previous.oaa AS difference
		FROM outs_above_average AS current
		JOIN outs_above_average AS previous
		ON current.player_id = previous.player_id
		WHERE current.date = date((SELECT MAX(date) FROM outs_above_average), '-1 day')
		AND previous.date = date((SELECT MAX(date) FROM outs_above_average WHERE date < (SELECT MAX(date) FROM outs_above_average)), '-1 day')
		AND current.oaa != previous.oaa
		ORDER BY ABS(difference) DESC
		LIMIT ?;
		`, limit)
	} else {
		// Otherwise, fetch the differences between the current and previous OAA totals starting from the previous day
		rows, err = db.Query(`
		SELECT 
			current.player_id,
			current.full_name,
			current.team,
			current.oaa AS current_oaa,
			previous.oaa AS previous_oaa,
			current.oaa - previous.oaa AS difference
		FROM outs_above_average AS current
		JOIN outs_above_average AS previous
		ON current.player_id = previous.player_id
		WHERE current.date = (SELECT MAX(date) FROM outs_above_average)
		AND previous.date = (SELECT MAX(date) FROM outs_above_average WHERE date < (SELECT MAX(date) FROM outs_above_average))
		AND current.oaa != previous.oaa
		ORDER BY ABS(difference) DESC
		LIMIT ?;`, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var differences []PlayerDifference
	for rows.Next() {
		var difference PlayerDifference
		if err := rows.Scan(&difference.PlayerID, &difference.Name, &difference.Team, &difference.CurrentOAA, &difference.PreviousOAA, &difference.Difference); err != nil {
			return nil, err
		}
		differences = append(differences, difference)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return differences, nil
}
