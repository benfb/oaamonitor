package models

import (
	"database/sql"
	"fmt"
)

// FetchPlayerDifferences retrieves the players with the biggest differences between the current and previous OAA totals from the database.
func FetchPlayerDifferences(db *sql.DB, limit int) ([]PlayerDifference, error) {
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

	if count == 0 {
		rows, err = db.Query(`
			WITH differences AS (
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
				WHERE current.date = (SELECT MAX(date) FROM outs_above_average WHERE date < (SELECT MAX(date) FROM outs_above_average))
				AND previous.date = (SELECT MAX(date) FROM outs_above_average WHERE date < (SELECT MAX(date) FROM outs_above_average WHERE date < (SELECT MAX(date) FROM outs_above_average)))
				AND current.oaa != previous.oaa
			),
			ranked AS (
				SELECT *,
					ROW_NUMBER() OVER (
						PARTITION BY CASE WHEN difference > 0 THEN 1 ELSE -1 END
						ORDER BY ABS(difference) DESC, difference DESC
					) AS movement_rank
				FROM differences
			)
			SELECT
				player_id,
				full_name,
				team,
				position,
				current_oaa,
				previous_oaa,
				difference
			FROM ranked
			WHERE movement_rank <= ?
			ORDER BY difference DESC;
			`, limit)
	} else {
		rows, err = db.Query(`
			WITH differences AS (
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
			),
			ranked AS (
				SELECT *,
					ROW_NUMBER() OVER (
						PARTITION BY CASE WHEN difference > 0 THEN 1 ELSE -1 END
						ORDER BY ABS(difference) DESC, difference DESC
					) AS movement_rank
				FROM differences
			)
			SELECT
				player_id,
				full_name,
				team,
				position,
				current_oaa,
				previous_oaa,
				difference
			FROM ranked
			WHERE movement_rank <= ?
			ORDER BY difference DESC;`, limit)
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
	query := `
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
		WHERE o.date >= (SELECT DATE(MAX(date), ?) FROM outs_above_average)
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
	),
	trends AS (
		SELECT
			player_id,
			full_name,
			team,
			position,
			start_oaa,
			end_oaa,
			(end_oaa - start_oaa) as difference
		FROM player_trends
		WHERE rn = 1 AND ABS(end_oaa - start_oaa) > ?
	),
	ranked AS (
		SELECT *,
			ROW_NUMBER() OVER (
				PARTITION BY CASE WHEN difference > 0 THEN 1 ELSE -1 END
				ORDER BY ABS(difference) DESC, difference DESC
			) AS movement_rank
		FROM trends
	)
	SELECT
		player_id,
		full_name,
		team,
		position,
		start_oaa,
		end_oaa,
		difference
	FROM ranked
	WHERE movement_rank <= 100
	ORDER BY difference DESC;
	`
	rows, err := db.Query(query, fmt.Sprintf("-%d days", daysAgo), threshold)
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
