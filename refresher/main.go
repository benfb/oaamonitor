package refresher

import (
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
	"github.com/benfb/oaamonitor/storage"
	_ "github.com/mattn/go-sqlite3"
)

func RunPeriodically(ctx context.Context, cfg *config.Config, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			log.Println("Refreshing data...")
			if err := GetLatestOAA(cfg); err != nil {
				log.Printf("Error refreshing data: %v", err)
			}
		}
	}
}

func GetLatestOAA(cfg *config.Config) error {
	currentYear := time.Now().Year()
	url := fmt.Sprintf("https://baseballsavant.mlb.com/leaderboard/outs_above_average?type=Fielder&startYear=%d&endYear=%d&split=yes&team=&range=year&min=10&pos=&roles=&viz=hide&csv=true", currentYear, currentYear)

	tmpFile, err := downloadFile(url)
	if err != nil {
		return fmt.Errorf("failed to download CSV file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if err := processCSV(tmpFile.Name(), cfg.DatabasePath); err != nil {
		return fmt.Errorf("failed to process CSV file: %v", err)
	}

	log.Println("CSV data successfully inserted or updated in the database.")

	if cfg.UploadDatabase {
		if err := storage.UploadDatabase(cfg.DatabasePath); err != nil {
			return fmt.Errorf("failed to upload database: %v", err)
		}
		log.Println("Database successfully uploaded to Fly Storage.")
	}

	return nil
}

func downloadFile(url string) (*os.File, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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

func processCSV(filepath, dbPath string) error {
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

	expectedFields := 16
	if len(header) != expectedFields {
		return fmt.Errorf("unexpected number of header fields: got %d, want %d", len(header), expectedFields)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := createTable(db); err != nil {
		return err
	}

	stmt, err := prepareInsertStatement(db)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for {
		record, err := reader.Read()
		log.Println(record)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading CSV record: %v", err)
		}

		if err := processRecord(stmt, record, expectedFields); err != nil {
			log.Printf("Error processing record: %v", err)
			continue
		}
	}

	return nil
}

func createTable(db *sql.DB) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS outs_above_average (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		player_id INTEGER,
		first_name TEXT,
		last_name TEXT,
		full_name TEXT,
		team TEXT,
		oaa INTEGER,
		actual_success_rate REAL,
		estimated_success_rate REAL,
		diff_success_rate REAL,
		date DATE DEFAULT CURRENT_DATE,
		UNIQUE(player_id, date)
	);`
	_, err := db.Exec(createTableSQL)
	return err
}

func prepareInsertStatement(db *sql.DB) (*sql.Stmt, error) {
	insertSQL := `
	INSERT INTO outs_above_average (player_id, first_name, last_name, full_name, team, oaa, actual_success_rate, estimated_success_rate, diff_success_rate, date)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_DATE)
	ON CONFLICT(player_id, date) DO UPDATE SET
		first_name = excluded.first_name,
		last_name = excluded.last_name,
		full_name = excluded.full_name,
		team = excluded.team,
		oaa = excluded.oaa,
		actual_success_rate = excluded.actual_success_rate,
		estimated_success_rate = excluded.estimated_success_rate,
		diff_success_rate = excluded.diff_success_rate
	`
	return db.Prepare(insertSQL)
}

func processRecord(stmt *sql.Stmt, record []string, expectedFields int) error {
	if len(record) != expectedFields {
		return fmt.Errorf("unexpected number of fields: got %d, want %d", len(record), expectedFields)
	}

	nameParts := strings.Split(record[0], ",")
	if len(nameParts) != 2 {
		return fmt.Errorf("invalid name format: %v", record[0])
	}
	firstName := strings.TrimSpace(nameParts[1])
	lastName := strings.TrimSpace(nameParts[0])
	fullName := fmt.Sprintf("%s %s", firstName, lastName)

	playerID, err := strconv.Atoi(record[1])
	if err != nil {
		return fmt.Errorf("invalid player ID: %v", record[1])
	}

	team := record[2]
	oaa, err := strconv.Atoi(record[6])
	if err != nil {
		return fmt.Errorf("invalid OAA value: %v", record[6])
	}

	actualSuccessRate, err := parsePercentage(record[13])
	if err != nil {
		return fmt.Errorf("invalid actual success rate: %v", record[13])
	}

	estimatedSuccessRate, err := parsePercentage(record[14])
	if err != nil {
		return fmt.Errorf("invalid estimated success rate: %v", record[14])
	}

	diffSuccessRate, err := parsePercentage(record[15])
	if err != nil {
		return fmt.Errorf("invalid diff success rate: %v", record[15])
	}

	_, err = stmt.Exec(playerID, firstName, lastName, fullName, team, oaa, actualSuccessRate, estimatedSuccessRate, diffSuccessRate)
	return err
}

func parsePercentage(percentageStr string) (float64, error) {
	percentageStr = strings.TrimSuffix(percentageStr, "%")
	percentageValue, err := strconv.ParseFloat(percentageStr, 64)
	if err != nil {
		return 0, err
	}
	if strings.HasSuffix(percentageStr, "-") {
		return percentageValue, nil
	}
	return percentageValue / 100, nil
}

func removeBOM(r io.Reader) io.Reader {
	b := make([]byte, 3)
	if _, err := r.Read(b); err != nil {
		return r
	}

	if b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return r
	}

	return io.MultiReader(strings.NewReader(string(b)), r)
}
