package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

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

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		info, err := entry.Info()
		if err != nil {
			return err
		}

		switch {
		case info.IsDir():
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		default:
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
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

	data, err := json.MarshalIndent(entries, "", "  ")
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

	if err := os.RemoveAll(cfgFlags.outputDir); err != nil {
		log.Fatalf("failed to prepare output directory: %v", err)
	}
	if err := os.MkdirAll(cfgFlags.outputDir, 0o755); err != nil {
		log.Fatalf("failed to create output directory: %v", err)
	}

	cfg := config.NewConfig()
	cfg.DownloadDatabase = false
	cfg.UploadDatabase = false
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

	if _, err := db.Exec("PRAGMA wal_checkpoint(FULL)"); err != nil {
		log.Printf("warning: unable to checkpoint WAL file: %v", err)
	}

	siteBuilder, err := site.NewBuilder(db, cfg)
	if err != nil {
		log.Fatalf("failed to create site builder: %v", err)
	}

	// Render index page
	indexHTML, err := siteBuilder.RenderIndex()
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

	players, err := models.FetchPlayers(db.DB)
	if err != nil {
		log.Fatalf("failed to fetch players: %v", err)
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

	if err := copyDir("static", filepath.Join(cfgFlags.outputDir, "static")); err != nil {
		log.Fatalf("failed to copy static assets: %v", err)
	}

	if err := db.Close(); err != nil {
		log.Fatalf("failed to close database: %v", err)
	}

	downloadDir := filepath.Join(cfgFlags.outputDir, "downloads")
	if err := copyFile(cfg.DatabasePath, filepath.Join(downloadDir, "oaamonitor.db")); err != nil {
		log.Fatalf("failed to copy database file: %v", err)
	}

	if err := buildSearchIndex(cfgFlags.outputDir, players); err != nil {
		log.Fatalf("failed to build search index: %v", err)
	}

	log.Println("Static site successfully generated in", cfgFlags.outputDir)
}
