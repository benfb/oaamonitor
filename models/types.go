package models

import "time"

// Player represents a player with their ID and name.
type Player struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Stat represents a row in the outs_above_average table.
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

// PlayerDifference represents a player's OAA trend over a time period.
type PlayerDifference struct {
	PlayerID    int    `json:"player_id"`
	Name        string `json:"name"`
	Team        string `json:"team"`
	Position    string `json:"position"`
	CurrentOAA  int    `json:"current_oaa"`
	PreviousOAA int    `json:"previous_oaa"`
	Difference  int    `json:"difference"`
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
