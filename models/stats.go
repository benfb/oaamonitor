package models

import "database/sql"

// FetchStats retrieves all OAA snapshots ordered for build-time grouping.
func FetchStats(db *sql.DB) ([]Stat, error) {
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
	ORDER BY player_id, date;`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []Stat
	for rows.Next() {
		var stat Stat
		if err := rows.Scan(&stat.PlayerID, &stat.Name, &stat.Team, &stat.Position, &stat.OAA, &stat.Date, &stat.ActualSuccessRate, &stat.EstimatedSuccessRate, &stat.DiffSuccessRate); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}
