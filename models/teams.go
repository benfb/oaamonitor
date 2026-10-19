package models

import (
	"database/sql"
	"sort"
	"time"
)

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
		if playerStatsList[i].LatestOAA != playerStatsList[j].LatestOAA {
			return playerStatsList[i].LatestOAA > playerStatsList[j].LatestOAA
		}
		if playerStatsList[i].Name != playerStatsList[j].Name {
			return playerStatsList[i].Name < playerStatsList[j].Name
		}
		return playerStatsList[i].PlayerID < playerStatsList[j].PlayerID
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
