package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func weekly(year, from, to int) []string {
	var names []string
	for w := from; w <= to; w++ {
		names = append(names, fmt.Sprintf("%d-W%02d-oaamonitor.db", year, w))
	}
	return names
}

func TestPruneKeepsEverythingWhenFewBackups(t *testing.T) {
	if got := prune(weekly(2026, 43, 46), 12); len(got) != 0 {
		t.Fatalf("expected nothing to prune, got %v", got)
	}
}

func TestPruneKeepsRecentWeeksAndOldestPerMonth(t *testing.T) {
	// 2026-W01..W30. Keep W19..W30 (newest 12) plus each month's first week.
	got := prune(weekly(2026, 1, 30), 12)

	// ISO week -> month by Thursday: W01 Jan, W05 Jan 29 (Jan), W06 Feb,
	// W10 Mar, W14 Apr, W19 May (already recent).
	monthly := []string{"2026-W01", "2026-W06", "2026-W10", "2026-W14"}
	for _, name := range got {
		for _, m := range monthly {
			if strings.HasPrefix(name, m) {
				t.Errorf("pruned monthly backup %s", name)
			}
		}
	}
	for _, name := range weekly(2026, 19, 30) {
		if slices.Contains(got, name) {
			t.Errorf("pruned recent backup %s", name)
		}
	}
	if want := 18 - len(monthly); len(got) != want {
		t.Errorf("pruned %d backups, want %d: %v", len(got), want, got)
	}
}

func TestPruneIgnoresUnrecognizedNames(t *testing.T) {
	names := append(weekly(2025, 1, 40), "notes.txt", "2025-W99-oaamonitor.db", "2026-W53-oaamonitor.db")
	for _, name := range prune(names, 1) {
		if !strings.HasSuffix(name, "-oaamonitor.db") || name == "2025-W99-oaamonitor.db" || name == "2026-W53-oaamonitor.db" {
			t.Errorf("pruned unrecognized name %s", name)
		}
	}
}

func TestParseWeekAcrossYearBoundary(t *testing.T) {
	// 2026-W01 starts Monday 2025-12-29; 2020 had 53 ISO weeks.
	cases := map[string]string{
		"2026-W01-oaamonitor.db": "2025-12-29",
		"2020-W53-oaamonitor.db": "2020-12-28",
	}
	for name, want := range cases {
		got, ok := parseWeek(name)
		if !ok || got.Format("2006-01-02") != want {
			t.Errorf("parseWeek(%q) = %v, %v; want %s", name, got, ok, want)
		}
	}
}
