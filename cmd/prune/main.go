// Command prune decides which weekly database backups to delete.
//
// It reads backup object names (one per line, e.g. "2026-W45-oaamonitor.db")
// from stdin and prints the names that should be deleted. It never deletes
// anything itself. Retention: the newest -keep-weekly backups, plus the oldest
// backup of every calendar month, are kept forever. Names that don't match the
// backup pattern are ignored and therefore never deleted.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var backupName = regexp.MustCompile(`^(\d{4})-W(\d{2})-oaamonitor\.db$`)

type backup struct {
	name string
	// week is the Monday that starts the ISO week.
	week time.Time
}

func main() {
	keepWeekly := flag.Int("keep-weekly", 12, "number of most recent weekly backups to keep")
	flag.Parse()
	if *keepWeekly < 1 {
		log.Fatal("-keep-weekly must be at least 1")
	}

	names, err := readNames(os.Stdin)
	if err != nil {
		log.Fatalf("Failed to read backup names: %v", err)
	}
	for _, name := range prune(names, *keepWeekly) {
		fmt.Println(name)
	}
}

func readNames(r io.Reader) ([]string, error) {
	var names []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if name := strings.TrimSpace(scanner.Text()); name != "" {
			names = append(names, name)
		}
	}
	return names, scanner.Err()
}

// prune returns the backup names to delete, oldest first.
func prune(names []string, keepWeekly int) []string {
	var backups []backup
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		if week, ok := parseWeek(name); ok {
			backups = append(backups, backup{name: name, week: week})
		}
	}
	sort.Slice(backups, func(i, j int) bool { return backups[i].week.Before(backups[j].week) })

	keep := map[string]bool{}
	for i := max(0, len(backups)-keepWeekly); i < len(backups); i++ {
		keep[backups[i].name] = true
	}
	// Backups are sorted oldest first, so the first one seen for a month is
	// that month's oldest. A week belongs to the month containing its Thursday,
	// matching how ISO weeks are assigned to years.
	months := map[string]bool{}
	for _, b := range backups {
		month := b.week.AddDate(0, 0, 3).Format("2006-01")
		if !months[month] {
			months[month] = true
			keep[b.name] = true
		}
	}

	var remove []string
	for _, b := range backups {
		if !keep[b.name] {
			remove = append(remove, b.name)
		}
	}
	return remove
}

// parseWeek returns the Monday of the ISO week named by a backup file.
func parseWeek(name string) (time.Time, bool) {
	m := backupName.FindStringSubmatch(name)
	if m == nil {
		return time.Time{}, false
	}
	year, _ := strconv.Atoi(m[1])
	week, _ := strconv.Atoi(m[2])
	if week < 1 || week > 53 {
		return time.Time{}, false
	}
	// January 4 is always in ISO week 1.
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	offset := (int(jan4.Weekday()) + 6) % 7 // days since Monday
	monday := jan4.AddDate(0, 0, -offset+(week-1)*7)
	if y, w := monday.ISOWeek(); y != year || w != week {
		return time.Time{}, false // e.g. W53 in a 52-week year
	}
	return monday, true
}
