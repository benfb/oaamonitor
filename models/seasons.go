package models

import "fmt"

func seasonDateRange(season int) (string, string) {
	return fmt.Sprintf("%04d-01-01", season), fmt.Sprintf("%04d-01-01", season+1)
}
