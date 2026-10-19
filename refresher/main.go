package refresher

import (
	"bufio"
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

	"github.com/benfb/oaamonitor/database"
)

// maxSkippedRowFraction is the share of CSV rows that may be malformed before the
// whole refresh is rejected.
const maxSkippedRowFraction = 0.05

// snapshotLocation is the timezone used to label daily snapshots, so a late-evening
// run is recorded under the baseball day it belongs to rather than the UTC date.
var snapshotLocation = loadSnapshotLocation()

func loadSnapshotLocation() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.UTC
	}
	return loc
}

func snapshotDate() string {
	return time.Now().In(snapshotLocation).Format("2006-01-02")
}

// GetLatestOAA downloads the current season's OAA leaderboard into the SQLite database at dbPath,
// then compacts, checkpoints, and verifies the file so it can be uploaded as-is.
func GetLatestOAA(ctx context.Context, dbPath string, timeout time.Duration) error {
	currentYear := time.Now().Year()
	url := fmt.Sprintf("https://baseballsavant.mlb.com/leaderboard/outs_above_average?type=Fielder&startYear=%d&endYear=%d&split=yes&team=&range=year&min=10&pos=&roles=&viz=hide&csv=true", currentYear, currentYear)

	tmpFile, err := downloadFile(ctx, url, timeout)
	if err != nil {
		return fmt.Errorf("failed to download CSV file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if err := processCSV(ctx, tmpFile.Name(), dbPath); err != nil {
		return fmt.Errorf("failed to process CSV file: %v", err)
	}

	log.Println("CSV data successfully inserted or updated in the database.")

	if err := compact(dbPath); err != nil {
		return fmt.Errorf("failed to compact database: %v", err)
	}

	// Leave a self-contained, verified file behind so it can be uploaded as-is.
	if err := database.CheckpointWAL(dbPath); err != nil {
		return fmt.Errorf("failed to checkpoint WAL: %v", err)
	}
	if err := database.Validate(dbPath); err != nil {
		return fmt.Errorf("database failed validation after refresh: %w", err)
	}

	return nil
}

func compact(dbPath string) error {
	db, err := database.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	compacted, err := database.CompactIfFragmented(db)
	if compacted {
		log.Println("Database compacted.")
	}
	return err
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

	date := snapshotDate()
	var total, skipped int
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading CSV record: %v", err)
		}

		total++
		if err := processRecord(stmt, record, columns, date); err != nil {
			skipped++
			log.Printf("skipping record %q: %v", columns.value(record, csvColumnName), err)
		}
	}
	if float64(skipped) > float64(total)*maxSkippedRowFraction {
		return fmt.Errorf("too many malformed records: skipped %d of %d", skipped, total)
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
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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

// csvColumn is the exact header Baseball Savant uses for a column in the OAA leaderboard CSV.
type csvColumn string

const (
	csvColumnName                 csvColumn = "last_name, first_name"
	csvColumnPlayerID             csvColumn = "player_id"
	csvColumnTeam                 csvColumn = "display_team_name"
	csvColumnPrimaryPosition      csvColumn = "primary_pos_formatted"
	csvColumnOAA                  csvColumn = "outs_above_average"
	csvColumnActualSuccessRate    csvColumn = "actual_success_rate_formatted"
	csvColumnEstimatedSuccessRate csvColumn = "adj_estimated_success_rate_formatted"
	csvColumnDiffSuccessRate      csvColumn = "diff_success_rate_formatted"
)

var requiredCSVColumns = []csvColumn{
	csvColumnName,
	csvColumnPlayerID,
	csvColumnTeam,
	csvColumnPrimaryPosition,
	csvColumnOAA,
	csvColumnActualSuccessRate,
	csvColumnEstimatedSuccessRate,
	csvColumnDiffSuccessRate,
}

// csvColumns maps each required column to its index in the CSV header.
type csvColumns map[csvColumn]int

// resolveCSVColumns locates every required column by exact name, so a renamed or dropped
// column fails the refresh instead of silently importing the wrong data.
func resolveCSVColumns(header []string) (csvColumns, error) {
	indexes := make(map[string]int, len(header))
	for i, name := range header {
		indexes[strings.TrimSpace(name)] = i
	}

	columns := make(csvColumns, len(requiredCSVColumns))
	for _, column := range requiredCSVColumns {
		index, ok := indexes[string(column)]
		if !ok {
			return nil, fmt.Errorf("CSV header missing required column %q; got %v", column, header)
		}
		columns[column] = index
	}
	return columns, nil
}

func (columns csvColumns) value(record []string, column csvColumn) string {
	index, ok := columns[column]
	if !ok || index < 0 || index >= len(record) {
		return ""
	}
	return record[index]
}

func processRecord(stmt *sql.Stmt, record []string, columns csvColumns, date string) error {
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

	_, err = stmt.Exec(playerID, firstName, lastName, fullName, team, primaryPosition, oaa, actualSuccessRate, estimatedSuccessRate, diffSuccessRate, date)
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
	br := bufio.NewReader(r)
	if prefix, err := br.Peek(3); err == nil && bytes.Equal(prefix, []byte{0xEF, 0xBB, 0xBF}) {
		br.Discard(3)
	}
	return br
}
