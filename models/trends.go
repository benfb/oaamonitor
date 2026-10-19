package models

import (
	"database/sql"
	"fmt"
	"time"
)

// FetchPlayerDifferences retrieves the players with the biggest differences between the latest and
// previous day's OAA snapshots. If nothing changed between those two snapshots, it falls back to the
// previous pair so the page is never empty after a no-op refresh. A pair is only compared when its
// snapshots are on consecutive days, so a gap in the data never shows up as a "daily" change.
func FetchPlayerDifferences(db *sql.DB, limit int) ([]PlayerDifference, error) {
	dates, err := latestSnapshotDates(db, 3)
	if err != nil {
		return nil, err
	}
	if len(dates) < 2 || !consecutiveDays(dates[1], dates[0]) {
		return nil, nil
	}

	differences, err := fetchDifferencesBetween(db, dates[0], dates[1], limit)
	if err != nil || len(differences) > 0 || len(dates) < 3 || !consecutiveDays(dates[2], dates[1]) {
		return differences, err
	}
	return fetchDifferencesBetween(db, dates[1], dates[2], limit)
}

// consecutiveDays reports whether later is exactly one calendar day after earlier.
func consecutiveDays(earlier, later string) bool {
	e, err := parseSnapshotDate(earlier)
	if err != nil {
		return false
	}
	l, err := parseSnapshotDate(later)
	if err != nil {
		return false
	}
	return e.AddDate(0, 0, 1).Equal(l)
}

func parseSnapshotDate(date string) (time.Time, error) {
	if len(date) > len("2006-01-02") {
		date = date[:len("2006-01-02")]
	}
	return time.Parse("2006-01-02", date)
}

func latestSnapshotDates(db *sql.DB, n int) ([]string, error) {
	rows, err := db.Query("SELECT DISTINCT date FROM outs_above_average ORDER BY date DESC LIMIT ?", n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var date string
		if err := rows.Scan(&date); err != nil {
			return nil, err
		}
		dates = append(dates, date)
	}
	return dates, rows.Err()
}

func fetchDifferencesBetween(db *sql.DB, currentDate, previousDate string, limit int) ([]PlayerDifference, error) {
	rows, err := db.Query(`
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
			WHERE current.date = ?
			AND previous.date = ?
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
		ORDER BY difference DESC;`, currentDate, previousDate, limit)
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
	return differences, rows.Err()
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
			ROW_NUMBER() OVER (PARTITION BY player_id ORDER BY date DESC) as rn
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
