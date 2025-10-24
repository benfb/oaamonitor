package site

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/benfb/oaamonitor/config"
	"github.com/benfb/oaamonitor/database"
	"github.com/benfb/oaamonitor/models"
	"github.com/benfb/oaamonitor/renderer"
)

// Team represents metadata needed to render navigation links and locate data.
type Team struct {
	Name       string
	Slug       string
	normalized string
}

// Builder prepares view data and renders templates for the static site.
type Builder struct {
	db          *database.DB
	cfg         *config.Config
	renderer    *renderer.Renderer
	teams       []Team
	teamsLoaded bool
}

// NewBuilder constructs a Builder backed by the provided database and config.
func NewBuilder(db *database.DB, cfg *config.Config) (*Builder, error) {
	r, err := renderer.New()
	if err != nil {
		return nil, err
	}

	return &Builder{
		db:       db,
		cfg:      cfg,
		renderer: r,
	}, nil
}

func (b *Builder) loadTeams() ([]Team, error) {
	if b.teamsLoaded {
		return b.teams, nil
	}

	names, err := models.FetchTeams(b.db.DB)
	if err != nil {
		return nil, err
	}

	teams := make([]Team, 0, len(names))
	seen := make(map[string]struct{})
	for _, name := range names {
		lower := strings.ToLower(name)
		normalized := NormalizeTeamName(lower)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		teams = append(teams, Team{
			Name:       name,
			Slug:       slugifyTeamName(normalized),
			normalized: normalized,
		})
	}

	b.teams = teams
	b.teamsLoaded = true
	return teams, nil
}

// Teams returns a copy of the cached teams list for navigation and routing.
func (b *Builder) Teams() ([]Team, error) {
	teams, err := b.loadTeams()
	if err != nil {
		return nil, err
	}
	result := make([]Team, len(teams))
	copy(result, teams)
	return result, nil
}

// RenderIndex builds the home page HTML.
func (b *Builder) RenderIndex() (string, error) {
	players, err := models.FetchPlayers(b.db.DB)
	if err != nil {
		return "", err
	}

	teams, err := b.loadTeams()
	if err != nil {
		return "", err
	}

	playerDifferences, err := models.FetchPlayerDifferences(b.db.DB, 100)
	if err != nil {
		return "", err
	}

	sevenDayTrends, err := models.FetchNDayTrends(b.db.DB, 7, 1)
	if err != nil {
		return "", err
	}

	thirtyDayTrends, err := models.FetchNDayTrends(b.db.DB, 30, 3)
	if err != nil {
		return "", err
	}

	dbSize, err := database.GetDatabaseSize(b.cfg.DatabasePath)
	if err != nil {
		dbSize = "Unknown"
	}

	data := struct {
		Title             string
		Players           []models.Player
		Teams             []Team
		PlayerDifferences []models.PlayerDifference
		SevenDayTrends    []models.PlayerDifference
		ThirtyDayTrends   []models.PlayerDifference
		DatabaseSize      string
	}{
		Title:             "Outs Above Average Monitor",
		Players:           players,
		Teams:             teams,
		PlayerDifferences: playerDifferences,
		SevenDayTrends:    sevenDayTrends,
		ThirtyDayTrends:   thirtyDayTrends,
		DatabaseSize:      dbSize,
	}

	return b.renderer.RenderToString("index.html", data)
}

// RenderPlayer builds the player page HTML for the supplied player ID.
func (b *Builder) RenderPlayer(playerID int) (string, error) {
	playerSeasons, err := models.FetchPlayerSeasons(b.db.DB, playerID)
	if err != nil {
		return "", err
	}

	if len(playerSeasons) == 0 {
		playerSeasons = []int{time.Now().Year()}
	}

	selectedSeason := playerSeasons[0]

	playerStatsBySeason := make(map[int][]models.Stat)
	playerPositions := make(map[int]string)
	var playerName string

	for _, season := range playerSeasons {
		stats, name, position, err := models.FetchPlayerStats(b.db.DB, playerID, season)
		if err != nil {
			return "", err
		}
		playerStatsBySeason[season] = stats
		if position == "" {
			position = "N/A"
		}
		playerPositions[season] = position
		if playerName == "" && name != "" {
			playerName = name
		}
	}

	if _, ok := playerStatsBySeason[selectedSeason]; !ok {
		playerStatsBySeason[selectedSeason] = []models.Stat{}
	}
	if _, ok := playerPositions[selectedSeason]; !ok {
		playerPositions[selectedSeason] = "N/A"
	}

	teams, err := b.loadTeams()
	if err != nil {
		return "", err
	}

	selectedStats := playerStatsBySeason[selectedSeason]
	selectedPosition := playerPositions[selectedSeason]
	if selectedPosition == "" {
		selectedPosition = "N/A"
	}

	title := playerName + " Outs Above Average"
	if selectedPosition != "N/A" {
		title = fmt.Sprintf("%s (%s) Outs Above Average", playerName, selectedPosition)
	}

	data := struct {
		Title               string
		PlayerID            int
		PlayerName          string
		PlayerPosition      string
		PlayerStats         []models.Stat
		PlayerStatsBySeason map[int][]models.Stat
		PlayerPositions     map[int]string
		Teams               []Team
		Seasons             []int
		SelectedSeason      int
	}{
		Title:               title,
		PlayerID:            playerID,
		PlayerName:          playerName,
		PlayerPosition:      selectedPosition,
		PlayerStats:         selectedStats,
		PlayerStatsBySeason: playerStatsBySeason,
		PlayerPositions:     playerPositions,
		Teams:               teams,
		Seasons:             playerSeasons,
		SelectedSeason:      selectedSeason,
	}

	return b.renderer.RenderToString("player.html", data)
}

// RenderTeam builds the team page HTML for the supplied team metadata.
func (b *Builder) RenderTeam(team Team) (string, error) {
	teamSeasons, err := models.FetchTeamSeasons(b.db.DB, team.normalized)
	if err != nil {
		return "", err
	}

	if len(teamSeasons) == 0 {
		teamSeasons, err = models.FetchSeasons(b.db.DB)
		if err != nil {
			return "", err
		}
	}

	if len(teamSeasons) == 0 {
		teamSeasons = []int{time.Now().Year()}
	}

	selectedSeason := teamSeasons[0]

	teamStatsBySeason := make(map[int][]models.Stat)
	sparklinesBySeason := make(map[int][]models.PlayerStats)
	var capitalizedTeamName string

	for _, season := range teamSeasons {
		stats, name, err := models.FetchTeamStats(b.db.DB, team.normalized, season)
		if err != nil {
			return "", err
		}
		teamStatsBySeason[season] = stats
		sparklinesBySeason[season] = models.MapStatsByPlayerID(stats)

		if capitalizedTeamName == "" && name != "" {
			capitalizedTeamName = name
		}
	}

	if _, ok := teamStatsBySeason[selectedSeason]; !ok {
		teamStatsBySeason[selectedSeason] = []models.Stat{}
	}
	if _, ok := sparklinesBySeason[selectedSeason]; !ok {
		sparklinesBySeason[selectedSeason] = []models.PlayerStats{}
	}

	teams, err := b.loadTeams()
	if err != nil {
		return "", err
	}

	if capitalizedTeamName == "" {
		capitalizedTeamName = team.Name
	}
	if capitalizedTeamName == "" {
		capitalizedTeamName = strings.TrimSpace(team.normalized)
	}

	selectedTeamStats := teamStatsBySeason[selectedSeason]
	selectedSparklines := sparklinesBySeason[selectedSeason]

	data := struct {
		Title              string
		TeamName           string
		TeamStats          []models.Stat
		Teams              []Team
		SparklinesData     []models.PlayerStats
		CurrentYear        string
		TeamAbbreviation   string
		Seasons            []int
		SelectedSeason     int
		TeamStatsBySeason  map[int][]models.Stat
		SparklinesBySeason map[int][]models.PlayerStats
	}{
		Title:              fmt.Sprintf("%s Outs Above Average", capitalizedTeamName),
		TeamName:           capitalizedTeamName,
		TeamStats:          selectedTeamStats,
		Teams:              teams,
		SparklinesData:     selectedSparklines,
		CurrentYear:        strconv.Itoa(selectedSeason),
		TeamAbbreviation:   models.GetTeamAbbreviation(capitalizedTeamName),
		Seasons:            teamSeasons,
		SelectedSeason:     selectedSeason,
		TeamStatsBySeason:  teamStatsBySeason,
		SparklinesBySeason: sparklinesBySeason,
	}

	return b.renderer.RenderToString("team.html", data)
}

// NormalizeTeamName normalizes team names to a standard format used in queries.
func NormalizeTeamName(teamName string) string {
	switch teamName {
	case "diamondbacks", "dbacks":
		return "d-backs"
	case "blue-jays", "red-sox", "white-sox":
		return strings.ReplaceAll(teamName, "-", " ")
	case "blue+jays", "red+sox", "white+sox":
		return strings.ReplaceAll(teamName, "+", " ")
	case "bluejays":
		return "blue jays"
	case "redsox":
		return "red sox"
	case "whitesox":
		return "white sox"
	default:
		return teamName
	}
}

func slugifyTeamName(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, "'", "")
	slug = strings.ReplaceAll(slug, ".", "")
	slug = strings.ReplaceAll(slug, "&", "and")
	slug = strings.ReplaceAll(slug, "+", "-")
	slug = strings.ReplaceAll(slug, " ", "-")
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return strings.Trim(slug, "-")
}
