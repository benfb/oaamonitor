package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/benfb/oaamonitor/config"
	"github.com/benfb/oaamonitor/database"
	"github.com/benfb/oaamonitor/refresher"
	"github.com/benfb/oaamonitor/server"
	"github.com/benfb/oaamonitor/storage"
)

func main() {
	// Load configuration
	cfg := config.NewConfig()

	// Download database if configured
	if cfg.DownloadDatabase {
		log.Println("Downloading database from Fly Storage")
		if err := storage.DownloadDatabase(cfg.DatabasePath); err != nil {
			log.Fatalf("Failed to download database: %v", err)
		}
	}

	// Initialize database connection
	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create and initialize server
	srv, err := server.New(db, cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start HTTP server in a goroutine
	httpServer := &http.Server{
		Addr:         ":" + config.GetEnvValue("PORT", "8080"),
		Handler:      srv,
		ReadTimeout:  time.Duration(cfg.RequestTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.RequestTimeout) * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Println("Starting server on port", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	// Start periodic refresh in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		refresher.RunPeriodically(ctx, cfg, time.Duration(cfg.RefreshRate)*time.Second, refresher.GetLatestOAA)
	}()

	// Wait for interrupt signal
	<-stop
	log.Println("Shutting down server...")

	// Cancel the refresh goroutine
	cancel()

	// Shutdown the HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	// Wait for goroutines to finish
	wg.Wait()
	log.Println("Server exited properly")
}
