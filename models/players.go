package models

import (
	"database/sql"
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
