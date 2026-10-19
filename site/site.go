package site

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/benfb/oaamonitor/models"
	"github.com/benfb/oaamonitor/renderer"
)

// Team represents metadata needed to render navigation links and locate data.
type Team struct {
	Name       string
	Slug       string
	normalized string
}

// Tab is one table of player movement on the home page.
type Tab struct {
	ID    string
	Label string
	Rows  []models.PlayerDifference
}

// chartStat is the slice of a snapshot the page scripts actually plot. Every season of history is
// embedded in each page, so the payload stays small by leaving out fields the scripts never read.
type chartStat struct {
	PlayerID int       `json:"player_id,omitempty"`
	Name     string    `json:"name,omitempty"`
	Team     string    `json:"team,omitempty"`
	Date     time.Time `json:"date"`
	OAA      int       `json:"oaa"`
}

// playerChartStats keeps what the player chart needs: the team (for the title) and each point.
func playerChartStats(stats []models.Stat) []chartStat {
	result := make([]chartStat, len(stats))
	for i, stat := range stats {
		result[i] = chartStat{Team: stat.Team, Date: stat.Date, OAA: stat.OAA}
	}
	return result
}

// teamChartStats keeps what the team chart needs to group points into one line per player.
func teamChartStats(stats []models.Stat) []chartStat {
	result := make([]chartStat, len(stats))
	for i, stat := range stats {
		result[i] = chartStat{PlayerID: stat.PlayerID, Name: stat.Name, Date: stat.Date, OAA: stat.OAA}
	}
	return result
}

// Builder prepares view data and renders templates for the static site.
type Builder struct {
	db          *sql.DB
	renderer    *renderer.Renderer
	teams       []Team
	teamsLoaded bool
	stats       *statIndex
}

type statIndex struct {
	playerStats map[int][]models.Stat
	teamStats   map[string][]models.Stat
	seasons     []int
}

// NewBuilder constructs a Builder backed by the provided database and template files.
func NewBuilder(db *sql.DB, templates fs.FS) (*Builder, error) {
	r, err := renderer.New(templates)
	if err != nil {
		return nil, err
	}

	return &Builder{
		db:       db,
		renderer: r,
	}, nil
}

func (b *Builder) loadTeams() ([]Team, error) {
	if b.teamsLoaded {
		return b.teams, nil
	}

	names, err := models.FetchTeams(b.db)
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

func (b *Builder) loadStats() (*statIndex, error) {
	if b.stats != nil {
		return b.stats, nil
	}

	stats, err := models.FetchStats(b.db)
	if err != nil {
		return nil, err
	}

	index := &statIndex{
		playerStats: make(map[int][]models.Stat),
		teamStats:   make(map[string][]models.Stat),
	}
	seasons := make(map[int]struct{})

	for _, stat := range stats {
		index.playerStats[stat.PlayerID] = append(index.playerStats[stat.PlayerID], stat)

		normalizedTeam := NormalizeTeamName(strings.ToLower(stat.Team))
		index.teamStats[normalizedTeam] = append(index.teamStats[normalizedTeam], stat)

		seasons[stat.Date.Year()] = struct{}{}
	}

	index.seasons = sortedSeasons(seasons)
	b.stats = index
	return index, nil
}

func sortedSeasons(seasons map[int]struct{}) []int {
	result := make([]int, 0, len(seasons))
	for season := range seasons {
		result = append(result, season)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(result)))
	return result
}

func seasonsForStats(stats []models.Stat) []int {
	seasons := make(map[int]struct{})
	for _, stat := range stats {
		seasons[stat.Date.Year()] = struct{}{}
	}
	return sortedSeasons(seasons)
}

func groupStatsBySeason(stats []models.Stat) map[int][]models.Stat {
	result := make(map[int][]models.Stat)
	for _, stat := range stats {
		season := stat.Date.Year()
		result[season] = append(result[season], stat)
	}
	return result
}

func playerPosition(stats []models.Stat) string {
	position := "N/A"
	for _, stat := range stats {
		if stat.Position != "N/A" {
			position = stat.Position
		}
	}
	return position
}

func latestPlayerName(stats []models.Stat) string {
	name := ""
	for _, stat := range stats {
		if stat.Name != "" {
			name = stat.Name
		}
	}
	return name
}

func teamName(stats []models.Stat) string {
	name := ""
	for _, stat := range stats {
		if stat.Team != "" {
			name = stat.Team
		}
	}
	return name
}

func normalizeTeamSeasonPositions(stats []models.Stat) []models.Stat {
	type latestPosition struct {
		position string
		date     time.Time
	}

	positions := make(map[int]latestPosition)
	for _, stat := range stats {
		if stat.Position == "N/A" {
			continue
		}
		latest, ok := positions[stat.PlayerID]
		if !ok || stat.Date.After(latest.date) {
			positions[stat.PlayerID] = latestPosition{
				position: stat.Position,
				date:     stat.Date,
			}
		}
	}

	result := make([]models.Stat, 0, len(stats))
	for _, stat := range stats {
		if latest, ok := positions[stat.PlayerID]; ok {
			stat.Position = latest.position
		}
		result = append(result, stat)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == result[j].Name {
			return result[i].Date.Before(result[j].Date)
		}
		return result[i].Name < result[j].Name
	})

	return result
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
	teams, err := b.loadTeams()
	if err != nil {
		return "", err
	}

	playerDifferences, err := models.FetchPlayerDifferences(b.db, 100)
	if err != nil {
		return "", err
	}

	sevenDayTrends, err := models.FetchNDayTrends(b.db, 7, 1)
	if err != nil {
		return "", err
	}

	thirtyDayTrends, err := models.FetchNDayTrends(b.db, 30, 3)
	if err != nil {
		return "", err
	}

	latestSnapshotDate, err := models.FetchLatestSnapshotDate(b.db)
	if err != nil {
		return "", err
	}

	// Windows without enough history are left out rather than shown empty.
	var tabs []Tab
	for _, tab := range []Tab{
		{ID: "daily", Label: "Daily", Rows: playerDifferences},
		{ID: "weekly", Label: "Weekly", Rows: sevenDayTrends},
		{ID: "monthly", Label: "Monthly", Rows: thirtyDayTrends},
	} {
		if len(tab.Rows) > 0 {
			tabs = append(tabs, tab)
		}
	}

	data := struct {
		Title              string
		Teams              []Team
		Tabs               []Tab
		LatestSnapshotDate string
	}{
		Title:              "Outs Above Average Monitor",
		Teams:              teams,
		Tabs:               tabs,
		LatestSnapshotDate: latestSnapshotDate,
	}

	return b.renderer.RenderToString("index.html", data)
}

// RenderPlayer builds the player page HTML for the supplied player ID.
func (b *Builder) RenderPlayer(playerID int) (string, error) {
	index, err := b.loadStats()
	if err != nil {
		return "", err
	}

	playerStats := index.playerStats[playerID]
	playerSeasons := seasonsForStats(playerStats)
	if len(playerSeasons) == 0 {
		playerSeasons = []int{time.Now().Year()}
	}

	selectedSeason := playerSeasons[0]

	playerStatsBySeason := groupStatsBySeason(playerStats)
	playerPositions := make(map[int]string)
	chartStatsBySeason := make(map[int][]chartStat)
	var playerName string

	for _, season := range playerSeasons {
		stats := playerStatsBySeason[season]
		playerPositions[season] = playerPosition(stats)
		chartStatsBySeason[season] = playerChartStats(stats)
		if name := latestPlayerName(stats); playerName == "" && name != "" {
			playerName = name
		}
	}

	teams, err := b.loadTeams()
	if err != nil {
		return "", err
	}

	selectedPosition := playerPositions[selectedSeason]

	title := playerName + " Outs Above Average"
	if selectedPosition != "N/A" {
		title = fmt.Sprintf("%s (%s) Outs Above Average", playerName, selectedPosition)
	}

	data := struct {
		Title               string
		PlayerID            int
		PlayerName          string
		PlayerStatsBySeason map[int][]chartStat
		PlayerPositions     map[int]string
		Teams               []Team
		Seasons             []int
		SelectedSeason      int
	}{
		Title:               title,
		PlayerID:            playerID,
		PlayerName:          playerName,
		PlayerStatsBySeason: chartStatsBySeason,
		PlayerPositions:     playerPositions,
		Teams:               teams,
		Seasons:             playerSeasons,
		SelectedSeason:      selectedSeason,
	}

	return b.renderer.RenderToString("player.html", data)
}

// RenderTeam builds the team page HTML for the supplied team metadata.
func (b *Builder) RenderTeam(team Team) (string, error) {
	index, err := b.loadStats()
	if err != nil {
		return "", err
	}

	teamStats := index.teamStats[team.normalized]
	teamSeasons := seasonsForStats(teamStats)
	if len(teamSeasons) == 0 {
		teamSeasons = index.seasons
	}

	if len(teamSeasons) == 0 {
		teamSeasons = []int{time.Now().Year()}
	}

	selectedSeason := teamSeasons[0]

	teamStatsBySeason := groupStatsBySeason(teamStats)
	chartStatsBySeason := make(map[int][]chartStat)
	sparklinesBySeason := make(map[int][]models.PlayerStats)
	var capitalizedTeamName string

	for _, season := range teamSeasons {
		stats := normalizeTeamSeasonPositions(teamStatsBySeason[season])
		chartStatsBySeason[season] = teamChartStats(stats)
		sparklinesBySeason[season] = models.MapStatsByPlayerID(stats)

		if name := teamName(stats); capitalizedTeamName == "" && name != "" {
			capitalizedTeamName = name
		}
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

	data := struct {
		Title              string
		TeamName           string
		Teams              []Team
		SparklinesData     []models.PlayerStats
		CurrentYear        string
		TeamAbbreviation   string
		Seasons            []int
		SelectedSeason     int
		TeamStatsBySeason  map[int][]chartStat
		SparklinesBySeason map[int][]models.PlayerStats
	}{
		Title:              fmt.Sprintf("%s Outs Above Average", capitalizedTeamName),
		TeamName:           capitalizedTeamName,
		Teams:              teams,
		SparklinesData:     sparklinesBySeason[selectedSeason],
		CurrentYear:        strconv.Itoa(selectedSeason),
		TeamAbbreviation:   models.GetTeamAbbreviation(capitalizedTeamName),
		Seasons:            teamSeasons,
		SelectedSeason:     selectedSeason,
		TeamStatsBySeason:  chartStatsBySeason,
		SparklinesBySeason: sparklinesBySeason,
	}

	return b.renderer.RenderToString("team.html", data)
}

// NormalizeTeamName normalizes team names to a standard format used in queries.
func NormalizeTeamName(teamName string) string {
	switch teamName {
	case "diamondbacks", "dbacks":
		return "d-backs"
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
