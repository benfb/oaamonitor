package refresher

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/benfb/oaamonitor/config"
	"github.com/benfb/oaamonitor/database"
	"github.com/benfb/oaamonitor/storage"
)

func GetLatestOAA(ctx context.Context, cfg *config.Config) error {
	currentYear := time.Now().Year()
	url := fmt.Sprintf("https://baseballsavant.mlb.com/leaderboard/outs_above_average?type=Fielder&startYear=%d&endYear=%d&split=yes&team=&range=year&min=10&pos=&roles=&viz=hide&csv=true", currentYear, currentYear)

	timeout := time.Duration(cfg.RequestTimeout) * time.Second
	tmpFile, err := downloadFile(ctx, url, timeout)
	if err != nil {
		return fmt.Errorf("failed to download CSV file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if err := processCSV(ctx, tmpFile.Name(), cfg.DatabasePath); err != nil {
		return fmt.Errorf("failed to process CSV file: %v", err)
	}

	log.Println("CSV data successfully inserted or updated in the database.")

	if cfg.UploadDatabase {
		if err := database.CheckpointWAL(cfg.DatabasePath); err != nil {
			return fmt.Errorf("failed to checkpoint WAL before upload: %v", err)
		}
		if err := storage.UploadDatabase(ctx, cfg.DatabasePath); err != nil {
			return fmt.Errorf("failed to upload database: %v", err)
		}
		log.Println("Database successfully uploaded to object storage.")
	}

	return nil
}

func downloadFile(ctx context.Context, url string, timeout time.Duration) (*os.File, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("unexpected status %d fetching CSV: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	tmpFile, err := os.CreateTemp("", "outs_above_average_*.csv")
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, err
	}

	if err := tmpFile.Close(); err != nil {
		return nil, err
	}

	return tmpFile, nil
}

func processCSV(ctx context.Context, filepath, dbPath string) error {
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(removeBOM(file))
	reader.LazyQuotes = true

	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV header: %v", err)
	}

	columns, err := resolveCSVColumns(header)
	if err != nil {
		return err
	}

	db, err := database.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := database.EnsureSchema(db); err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	stmt, err := prepareInsertStatement(tx)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading CSV record: %v", err)
		}

		if err := processRecord(stmt, record, columns); err != nil {
			return fmt.Errorf("error processing record %v: %w", columns.value(record, csvColumnName), err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func prepareInsertStatement(db interface {
	Prepare(string) (*sql.Stmt, error)
}) (*sql.Stmt, error) {
	insertSQL := `
	INSERT INTO outs_above_average (player_id, first_name, last_name, full_name, team, primary_position, oaa, actual_success_rate, estimated_success_rate, diff_success_rate, date)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_DATE)
	ON CONFLICT(player_id, date) DO UPDATE SET
		first_name = excluded.first_name,
		last_name = excluded.last_name,
		full_name = excluded.full_name,
		team = excluded.team,
		primary_position = excluded.primary_position,
		oaa = excluded.oaa,
		actual_success_rate = excluded.actual_success_rate,
		estimated_success_rate = excluded.estimated_success_rate,
		diff_success_rate = excluded.diff_success_rate
	`
	return db.Prepare(insertSQL)
}

type csvColumn string

const (
	csvColumnName                 csvColumn = "name"
	csvColumnPlayerID             csvColumn = "player ID"
	csvColumnTeam                 csvColumn = "team"
	csvColumnPrimaryPosition      csvColumn = "primary position"
	csvColumnOAA                  csvColumn = "OAA"
	csvColumnActualSuccessRate    csvColumn = "actual success rate"
	csvColumnEstimatedSuccessRate csvColumn = "estimated success rate"
	csvColumnDiffSuccessRate      csvColumn = "diff success rate"
)

type csvColumns map[csvColumn]int

var requiredCSVColumns = map[csvColumn][]string{
	csvColumnName: {
		"name",
		"player",
		"playername",
		"last_name, first_name",
		"fielder",
		"fieldername",
		"fielder_name",
		"entityname",
	},
	csvColumnPlayerID: {
		"playerid",
		"player_id",
		"mlbid",
		"mlb_id",
	},
	csvColumnTeam: {
		"team",
		"teamname",
		"team_name",
		"display_team_name",
	},
	csvColumnPrimaryPosition: {
		"primaryposition",
		"primary_position",
		"primarypos",
		"primary_pos",
		"primaryposformatted",
		"primary_pos_formatted",
		"position",
		"pos",
	},
	csvColumnOAA: {
		"oaa",
		"outsaboveaverage",
		"outs_above_average",
	},
	csvColumnActualSuccessRate: {
		"actualsuccessrate",
		"actual_success_rate",
		"actualsuccessrateformatted",
		"actual_success_rate_formatted",
	},
	csvColumnEstimatedSuccessRate: {
		"estimatedsuccessrate",
		"estimated_success_rate",
		"estimatedsuccessrateformatted",
		"estimated_success_rate_formatted",
		"adj_estimated_success_rate_formatted",
	},
	csvColumnDiffSuccessRate: {
		"diffsuccessrate",
		"diff_success_rate",
		"diffsuccessrateformatted",
		"diff_success_rate_formatted",
		"differenceinsuccessrate",
		"difference_in_success_rate",
	},
}

func resolveCSVColumns(header []string) (csvColumns, error) {
	indexes := make(map[string]int, len(header))
	for i, name := range header {
		indexes[normalizeCSVHeader(name)] = i
	}

	columns := make(csvColumns, len(requiredCSVColumns))
	for column, aliases := range requiredCSVColumns {
		index, ok := findCSVColumn(indexes, aliases)
		if !ok {
			return nil, fmt.Errorf("CSV header missing required column %q; got %v", column, header)
		}
		columns[column] = index
	}

	return columns, nil
}

func findCSVColumn(indexes map[string]int, aliases []string) (int, bool) {
	for _, alias := range aliases {
		if index, ok := indexes[normalizeCSVHeader(alias)]; ok {
			return index, true
		}
	}
	return 0, false
}

func normalizeCSVHeader(header string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	var b strings.Builder
	for _, r := range header {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (columns csvColumns) value(record []string, column csvColumn) string {
	index, ok := columns[column]
	if !ok || index < 0 || index >= len(record) {
		return ""
	}
	return record[index]
}

func processRecord(stmt *sql.Stmt, record []string, columns csvColumns) error {
	name := columns.value(record, csvColumnName)
	if name == "" {
		return fmt.Errorf("missing %s", csvColumnName)
	}

	nameParts := strings.Split(name, ",")
	if len(nameParts) != 2 {
		return fmt.Errorf("invalid name format: %v", name)
	}
	firstName := strings.TrimSpace(nameParts[1])
	lastName := strings.TrimSpace(nameParts[0])
	fullName := fmt.Sprintf("%s %s", firstName, lastName)

	playerIDValue := columns.value(record, csvColumnPlayerID)
	playerID, err := strconv.Atoi(playerIDValue)
	if err != nil {
		return fmt.Errorf("invalid player ID: %v", playerIDValue)
	}

	team := columns.value(record, csvColumnTeam)

	oaaValue := columns.value(record, csvColumnOAA)
	oaa, err := strconv.Atoi(oaaValue)
	if err != nil {
		return fmt.Errorf("invalid OAA value: %v", oaaValue)
	}

	primaryPosition := columns.value(record, csvColumnPrimaryPosition)

	actualSuccessRateValue := columns.value(record, csvColumnActualSuccessRate)
	actualSuccessRate, err := parsePercentage(actualSuccessRateValue)
	if err != nil {
		return fmt.Errorf("invalid actual success rate: %v", actualSuccessRateValue)
	}

	estimatedSuccessRateValue := columns.value(record, csvColumnEstimatedSuccessRate)
	estimatedSuccessRate, err := parsePercentage(estimatedSuccessRateValue)
	if err != nil {
		return fmt.Errorf("invalid estimated success rate: %v", estimatedSuccessRateValue)
	}

	diffSuccessRateValue := columns.value(record, csvColumnDiffSuccessRate)
	diffSuccessRate, err := parsePercentage(diffSuccessRateValue)
	if err != nil {
		return fmt.Errorf("invalid diff success rate: %v", diffSuccessRateValue)
	}

	_, err = stmt.Exec(playerID, firstName, lastName, fullName, team, primaryPosition, oaa, actualSuccessRate, estimatedSuccessRate, diffSuccessRate)
	return err
}

func parsePercentage(percentageStr string) (float64, error) {
	percentageStr = strings.TrimSuffix(percentageStr, "%")
	percentageValue, err := strconv.ParseFloat(percentageStr, 64)
	if err != nil {
		return 0, err
	}
	return percentageValue / 100, nil
}

func removeBOM(r io.Reader) io.Reader {
	b := make([]byte, 3)
	n, err := r.Read(b)
	if err != nil {
		return r
	}

	if n == 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return r
	}

	return io.MultiReader(bytes.NewReader(b[:n]), r)
}
