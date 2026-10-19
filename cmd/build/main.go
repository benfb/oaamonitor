package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/benfb/oaamonitor"
	"github.com/benfb/oaamonitor/config"
	"github.com/benfb/oaamonitor/database"
	"github.com/benfb/oaamonitor/models"
	"github.com/benfb/oaamonitor/site"
)

type buildConfig struct {
	outputDir string
	database  string
}

func parseFlags() buildConfig {
	output := flag.String("out", "public", "output directory for generated site")
	database := flag.String("database", "", "path to the SQLite database (defaults to DATABASE_PATH env or ./data/oaamonitor.db)")
	flag.Parse()

	return buildConfig{
		outputDir: *output,
		database:  *database,
	}
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// copyAssets writes the files in src under dst, shipping only the minified build of
// any .js or .css asset that has one.
func copyAssets(src fs.FS, dst string) error {
	return fs.WalkDir(src, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || hasMinifiedSibling(src, name) {
			return nil
		}
		data, err := fs.ReadFile(src, name)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func hasMinifiedSibling(fsys fs.FS, name string) bool {
	ext := path.Ext(name)
	if ext != ".js" && ext != ".css" || strings.HasSuffix(name, ".min"+ext) {
		return false
	}
	_, err := fs.Stat(fsys, strings.TrimSuffix(name, ext)+".min"+ext)
	return err == nil
}

func validateOutputDir(path string) error {
	clean := filepath.Clean(path)
	if clean == "" || clean == "." || clean == string(os.PathSeparator) {
		return fmt.Errorf("refusing to remove unsafe output path %q", path)
	}
	if strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("output path %q resolves outside the working tree", path)
	}
	if filepath.IsAbs(clean) {
		return fmt.Errorf("output path %q must be a relative path", path)
	}
	return nil
}

func buildSearchIndex(outputDir string, players []models.Player) error {
	type searchEntry struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	entries := make([]searchEntry, 0, len(players))
	for _, player := range players {
		entries = append(entries, searchEntry{
			ID:   player.ID,
			Name: player.Name,
		})
	}

	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}

	target := filepath.Join(outputDir, "search-index.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}

func main() {
	cfgFlags := parseFlags()

	if err := validateOutputDir(cfgFlags.outputDir); err != nil {
		log.Fatalf("invalid output path: %v", err)
	}
	if err := os.RemoveAll(cfgFlags.outputDir); err != nil {
		log.Fatalf("failed to prepare output directory: %v", err)
	}
	if err := os.MkdirAll(cfgFlags.outputDir, 0o755); err != nil {
		log.Fatalf("failed to create output directory: %v", err)
	}

	cfg := config.NewConfig()
	if cfgFlags.database != "" {
		cfg.DatabasePath = cfgFlags.database
	}

	if info, err := os.Stat(cfg.DatabasePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Fatalf("database %s not found; run `go run ./cmd/fetch -database %s` first", cfg.DatabasePath, cfg.DatabasePath)
		}
		log.Fatalf("failed to stat database: %v", err)
	} else if info.IsDir() {
		log.Fatalf("database path %s is a directory, expected SQLite file", cfg.DatabasePath)
	}

	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	siteBuilder, err := site.NewBuilder(db, oaamonitor.Templates())
	if err != nil {
		log.Fatalf("failed to create site builder: %v", err)
	}

	players, err := models.FetchPlayers(db.DB)
	if err != nil {
		log.Fatalf("failed to fetch players: %v", err)
	}

	// Render index page
	indexHTML, err := siteBuilder.RenderIndex(players)
	if err != nil {
		log.Fatalf("failed to render index: %v", err)
	}
	if err := writeFile(filepath.Join(cfgFlags.outputDir, "index.html"), indexHTML); err != nil {
		log.Fatalf("failed to write index: %v", err)
	}

	teams, err := siteBuilder.Teams()
	if err != nil {
		log.Fatalf("failed to load teams: %v", err)
	}

	for _, player := range players {
		html, err := siteBuilder.RenderPlayer(player.ID)
		if err != nil {
			log.Fatalf("failed to render player %d: %v", player.ID, err)
		}
		target := filepath.Join(cfgFlags.outputDir, "player", fmt.Sprintf("%d", player.ID), "index.html")
		if err := writeFile(target, html); err != nil {
			log.Fatalf("failed to write player page for %d: %v", player.ID, err)
		}
	}

	for _, team := range teams {
		html, err := siteBuilder.RenderTeam(team)
		if err != nil {
			log.Fatalf("failed to render team %s: %v", team.Name, err)
		}
		target := filepath.Join(cfgFlags.outputDir, "team", team.Slug, "index.html")
		if err := writeFile(target, html); err != nil {
			log.Fatalf("failed to write team page for %s: %v", team.Name, err)
		}
	}

	if err := copyAssets(oaamonitor.Static(), filepath.Join(cfgFlags.outputDir, "static")); err != nil {
		log.Fatalf("failed to copy static assets: %v", err)
	}

	if err := db.Close(); err != nil {
		log.Fatalf("failed to close database: %v", err)
	}

	if err := buildSearchIndex(cfgFlags.outputDir, players); err != nil {
		log.Fatalf("failed to build search index: %v", err)
	}

	log.Println("Static site successfully generated in", cfgFlags.outputDir)
}
