package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"staccato/internal/config"
	"staccato/internal/database"
	"staccato/internal/server"
)

func main() {
	configPath := "./config.toml"

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// Check if music directory exists
	if _, err := os.Stat(cfg.Music.LibraryPath); os.IsNotExist(err) {
		log.Fatalf("Music directory '%s' does not exist. Please create it and add your music files.", cfg.Music.LibraryPath)
	}

	// Initialize database
	db, err := database.NewDatabase(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	defer db.Close()

	// Create and configure the music server
	musicServer, err := server.NewMusicServer(cfg, db)
	if err != nil {
		log.Fatalf("Error creating music server: %v", err)
	}

	// Scan the music library
	if err := musicServer.ScanMusicLibrary(); err != nil {
		log.Fatalf("Error scanning music library: %v", err)
	}

	// Check track count and warn if empty
	if cfg.Music.ScanOnStartup {
		tracks, err := db.GetAllTracks()
		if err != nil {
			log.Printf("Warning: Could not get track count: %v", err)
		} else if len(tracks) == 0 {
			log.Println("⚠️  No supported audio files found in music directory.")
			log.Printf("Supported formats: %v", cfg.Music.SupportedFormats)
		}
	}

	// Start the server
	musicServer.Start()

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("🛑 Received shutdown signal")
	musicServer.Shutdown()
}
