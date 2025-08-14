package main

import (
	"os"
	"os/signal"
	"syscall"

	"staccato/internal/config"
	"staccato/internal/database"
	"staccato/internal/server"

	"github.com/sirupsen/logrus"
)

func main() {
	configPath := "./config.toml"

	// Initialize basic logger for startup
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.WithError(err).Fatal("Error loading configuration")
	}

	// Check if music directory exists
	if _, err := os.Stat(cfg.Music.LibraryPath); os.IsNotExist(err) {
		logger.WithField("library_path", cfg.Music.LibraryPath).Fatal("Music directory does not exist. Please create it and add your music files.")
	}

	// Initialize database
	db, err := database.NewDatabase(cfg.Database.Path)
	if err != nil {
		logger.WithError(err).Fatal("Error initializing database")
	}
	defer db.Close()

	// Create and configure the music server
	musicServer, err := server.NewMusicServer(cfg, db)
	if err != nil {
		logger.WithError(err).Fatal("Error creating music server")
	}

	// Scan the music library
	if err := musicServer.ScanMusicLibrary(); err != nil {
		logger.WithError(err).Fatal("Error scanning music library")
	}

	// Check track count and warn if empty
	if cfg.Music.ScanOnStartup {
		tracks, err := db.GetAllTracks()
		if err != nil {
			logger.WithError(err).Warn("Could not get track count")
		} else if len(tracks) == 0 {
			logger.WithField("supported_formats", cfg.Music.SupportedFormats).Warn("No supported audio files found in music directory")
		}
	}

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Start the server in a goroutine
	go func() {
		musicServer.Start()
	}()

	// Wait for shutdown signal
	<-c

	logger.Info("Received shutdown signal")
	musicServer.Shutdown()
}
