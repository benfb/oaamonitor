package refresher

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/benfb/oaamonitor/config"
	"github.com/benfb/oaamonitor/storage"
	_ "github.com/mattn/go-sqlite3"
)

func GetLatestOAA() {
	// Get the current year
	currentYear := time.Now().Year()

	// Construct the URL
	url := fmt.Sprintf("https://baseballsavant.mlb.com/leaderboard/outs_above_average?type=Fielder&startYear=%d&endYear=%d&split=yes&team=&range=year&min=10&pos=&roles=&viz=hide&csv=true", currentYear, currentYear)

	// Download the CSV file to a temporary file
	tmpFile, err := downloadFile(url)
	if err != nil {
		fmt.Printf("Failed to download CSV file: %v\n", err)
		return
	}
	defer os.Remove(tmpFile.Name())

	cfg := config.NewConfig()

	// Process the CSV file and insert into the database
	err = processCSV(tmpFile.Name(), cfg.DatabasePath)
	if err != nil {
		fmt.Printf("Failed to process CSV file: %v\n", err)
		return
	}

	fmt.Println("CSV data successfully inserted or updated in the database.")
	// Find the SQLite database
	if cfg.UploadDatabase {
		storage.UploadDatabase(cfg.DatabasePath)
		fmt.Println("Database successfully uploaded to Fly Storage.")
	}
}

func downloadFile(url string) (*os.File, error) {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "outs_above_average_*.csv")
	if err != nil {
		return nil, err
	}

	// Write the body to file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, err
	}

	// Close the file to flush the write operations
	err = tmpFile.Close()
	if err != nil {
		return nil, err
	}

	return tmpFile, nil
}

func processCSV(filepath, dbPath string) error {
	// Open the CSV file
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read the CSV file and handle BOM
	reader := csv.NewReader(removeBOM(file))
	reader.LazyQuotes = true // Allow for lenient parsing of quotes

	// Read header row
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV header: %v", err)
	}
	fmt.Println("Header:", header)

	// Validate header fields
	expectedFields := 16
	if len(header) != expectedFields {
		return fmt.Errorf("unexpected number of header fields: got %d, want %d", len(header), expectedFields)
	}

	// Open the database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Create the table if it doesn't exist
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
	_, err = db.Exec(createTableSQL)
	if err != nil {
		return err
	}

	// Prepare the insert or replace statement
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
	stmt, err := db.Prepare(insertSQL)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Iterate through the records and insert or update in the database
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading CSV record: %v", err)
		}

		// Print each record for debugging
		fmt.Println("Record:", record)

		// Ensure the record has the expected number of fields
		if len(record) != expectedFields {
			fmt.Printf("Skipping record with unexpected number of fields: %v\n", record)
			continue
		}

		// Correct the first record field (name)
		nameParts := strings.Split(record[0], ",")
		if len(nameParts) != 2 {
			fmt.Printf("Skipping record with invalid name format: %v\n", record)
			continue
		}
		firstName := strings.TrimSpace(nameParts[1])
		lastName := strings.TrimSpace(nameParts[0])
		fullName := fmt.Sprintf("%s %s", firstName, lastName)

		// Extract other fields
		playerID, err := strconv.Atoi(record[1])
		if err != nil {
			fmt.Printf("Skipping record with invalid player ID: %v\n", record)
			continue
		}

		team := record[2]
		oaa, err := strconv.Atoi(record[6]) // Adjust based on the correct field index for OAA
		if err != nil {
			fmt.Printf("Skipping record with invalid OAA value: %v\n", record)
			continue
		}

		// Convert percentage strings to floats
		actualSuccessRate, err := parsePercentage(record[13])
		if err != nil {
			fmt.Printf("Skipping record with invalid actual success rate: %v\n", record)
			continue
		}

		estimatedSuccessRate, err := parsePercentage(record[14])
		if err != nil {
			fmt.Printf("Skipping record with invalid estimated success rate: %v\n", record)
			continue
		}

		diffSuccessRate, err := parsePercentage(record[15])
		if err != nil {
			fmt.Printf("Skipping record with invalid diff success rate: %v\n", record)
			continue
		}

		// Insert or update the record in the database
		_, err = stmt.Exec(playerID, firstName, lastName, fullName, team, oaa, actualSuccessRate, estimatedSuccessRate, diffSuccessRate)
		if err != nil {
			return err
		}
	}

	return nil
}

// parsePercentage converts a percentage string like "93%" or "-1%" to a float64
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
	_, err := r.Read(b)
	if err != nil {
		return r
	}

	if b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return r
	}

	return io.MultiReader(strings.NewReader(string(b)), r)
}
