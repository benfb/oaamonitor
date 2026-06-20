package models

import (
	"database/sql"
	"strconv"
)

// FetchPlayers retrieves distinct player IDs and names from the database in alphabetical order.
func FetchPlayers(db *sql.DB) ([]Player, error) {
	rows, err := db.Query(`
		SELECT player_id, full_name FROM outs_above_average
		WHERE (player_id, date) IN (SELECT player_id, MAX(date) FROM outs_above_average GROUP BY player_id)
		ORDER BY last_name, first_name`)
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

// FetchPlayerStats retrieves statistics for a specific player from the database.
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

	if len(stats) == 0 {
		var name, position string
		err := db.QueryRow(`
			SELECT full_name, COALESCE(primary_position, 'N/A')
			FROM outs_above_average
			WHERE player_id = ?
			ORDER BY date DESC LIMIT 1
		`, playerID).Scan(&name, &position)
		if err != nil && err != sql.ErrNoRows {
			return nil, "", "", err
		}
		if err != nil {
			return []Stat{}, "", "N/A", nil
		}
		return []Stat{}, name, position, nil
	}

	if playerPosition == "" {
		playerPosition = "N/A"
	}

	return stats, playerName, playerPosition, nil
}

// FetchPlayerSeasons returns the seasons in which the player has recorded stats.
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
