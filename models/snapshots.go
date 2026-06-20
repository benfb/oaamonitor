package models

import "database/sql"

// FetchLatestSnapshotDate returns the latest OAA snapshot date in YYYY-MM-DD format.
func FetchLatestSnapshotDate(db *sql.DB) (string, error) {
	var latest sql.NullString
	if err := db.QueryRow("SELECT MAX(date) FROM outs_above_average").Scan(&latest); err != nil {
		return "", err
	}
	if !latest.Valid {
		return "", nil
	}
	return latest.String, nil
}
