package models

import (
	"database/sql"
	"sort"
	"strconv"
	"strings"
	"time"
)

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

	if len(stats) == 0 {
		var teamNameResult string
		err := db.QueryRow(`
			SELECT team FROM outs_above_average
			WHERE LOWER(team) = ?
			LIMIT 1
		`, teamName).Scan(&teamNameResult)
		if err != nil && err != sql.ErrNoRows {
			return nil, "", err
		}
		if err == nil {
			capitalizedTeamName = teamNameResult
		}
	}

	return stats, capitalizedTeamName, nil
}

// MapStatsByPlayerID returns a slice of PlayerStats sorted by current OAA in descending order.
func MapStatsByPlayerID(stats []Stat) []PlayerStats {
	statsMap := make(map[int]*PlayerStats)
	latestDates := make(map[int]time.Time)

	for _, stat := range stats {
		if entry, ok := statsMap[stat.PlayerID]; !ok {
			statsMap[stat.PlayerID] = &PlayerStats{
				PlayerID:   stat.PlayerID,
				Name:       stat.Name,
				Position:   stat.Position,
				LatestOAA:  stat.OAA,
				OAAHistory: []SparklinePoint{{Date: stat.Date, OAA: stat.OAA}},
			}
			latestDates[stat.PlayerID] = stat.Date
		} else {
			if stat.Date.After(latestDates[stat.PlayerID]) {
				entry.LatestOAA = stat.OAA
				latestDates[stat.PlayerID] = stat.Date
			}

			entry.OAAHistory = append(entry.OAAHistory, SparklinePoint{Date: stat.Date, OAA: stat.OAA})

			if stat.Position != "N/A" {
				entry.Position = stat.Position
			}
		}
	}

	playerStatsList := make([]PlayerStats, 0, len(statsMap))
	for _, v := range statsMap {
		playerStatsList = append(playerStatsList, *v)
	}

	sort.Slice(playerStatsList, func(i, j int) bool {
		return playerStatsList[i].LatestOAA > playerStatsList[j].LatestOAA
	})

	return playerStatsList
}

// FetchTeams retrieves distinct team names from the database.
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

// GetTeamAbbreviation returns the MLB abbreviation for a given team name.
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

// FetchSeasons retrieves distinct seasons from the database.
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

// FetchTeamSeasons returns the seasons in which the team has recorded stats.
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
