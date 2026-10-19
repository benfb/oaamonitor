package site

import (
	"strings"
	"testing"

	"github.com/benfb/oaamonitor"
	"github.com/benfb/oaamonitor/database"
)

func TestBuilderRendersAllPageTypes(t *testing.T) {
	dbPath := t.TempDir() + "/oaamonitor.db"
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer db.Close()
	if err := database.EnsureSchema(db); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO outs_above_average
			(player_id, first_name, last_name, full_name, team, primary_position, oaa,
			 actual_success_rate, estimated_success_rate, diff_success_rate, date)
		VALUES
			(1, 'Jane', 'Doe', 'Jane Doe', 'Mets', 'SS', 2, 0.8, 0.7, 0.1, '2026-04-01'),
			(1, 'Jane', 'Doe', 'Jane Doe', 'Mets', 'SS', 4, 0.8, 0.7, 0.1, '2026-04-03')
	`)
	if err != nil {
		t.Fatalf("insert fixture data: %v", err)
	}

	builder, err := NewBuilder(db, oaamonitor.Templates())
	if err != nil {
		t.Fatalf("create builder: %v", err)
	}
	indexHTML, err := builder.RenderIndex()
	if err != nil {
		t.Fatalf("render index: %v", err)
	}
	if !strings.Contains(indexHTML, "Updated 2026-04-03") {
		t.Fatalf("index did not include latest snapshot date: %s", indexHTML)
	}

	playerHTML, err := builder.RenderPlayer(1)
	if err != nil {
		t.Fatalf("render player: %v", err)
	}
	if !strings.Contains(playerHTML, "Jane Doe") || !strings.Contains(playerHTML, "playerStatsBySeason") {
		t.Fatalf("player page missing expected data")
	}

	teams, err := builder.Teams()
	if err != nil {
		t.Fatalf("load teams: %v", err)
	}
	if len(teams) != 1 || teams[0].Slug != "mets" {
		t.Fatalf("unexpected teams: %#v", teams)
	}

	teamHTML, err := builder.RenderTeam(teams[0])
	if err != nil {
		t.Fatalf("render team: %v", err)
	}
	if !strings.Contains(teamHTML, "Mets Outs Above Average") || !strings.Contains(teamHTML, "sparklinesBySeason") {
		t.Fatalf("team page missing expected data")
	}
}
