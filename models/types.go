package models

import "database/sql"

// Player represents a player with their ID and name
type Player struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

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
	rows, err := db.Query("SELECT DISTINCT LOWER(team) FROM outs_above_average ORDER BY team")
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
