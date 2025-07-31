package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"spotigo/internal/config"
	"spotigo/internal/database"
	"spotigo/internal/downloader"
	"spotigo/internal/metadata"
	"spotigo/internal/ngrok"

	"github.com/fsnotify/fsnotify"
)

// MusicServer represents the main music streaming server
type MusicServer struct {
	db           *database.Database
	config       *config.Config
	watcher      *fsnotify.Watcher
	extractor    *metadata.Extractor
	downloader   *downloader.Downloader
	ngrokService *ngrok.Service
}

// NewMusicServer creates a new music server instance
func NewMusicServer(cfg *config.Config, db *database.Database) (*MusicServer, error) {
	// Create downloader
	dl, err := downloader.NewDownloader(cfg)
	if err != nil {
		log.Printf("Warning: Downloader not available: %v", err)
		dl = nil // Downloader will be nil if not available
	}

	// Create ngrok service
	ngrokSvc, err := ngrok.NewService(&cfg.Ngrok)
	if err != nil {
		log.Printf("Warning: Ngrok service not available: %v", err)
		ngrokSvc = nil
	}

	return &MusicServer{
		db:           db,
		config:       cfg,
		extractor:    metadata.NewExtractor(cfg.Music.SupportedFormats),
		downloader:   dl,
		ngrokService: ngrokSvc,
	}, nil
}

// ScanMusicLibrary scans the music directory and adds tracks to the database
func (ms *MusicServer) ScanMusicLibrary() error {
	if !ms.config.Music.ScanOnStartup {
		log.Println("Skipping library scan (disabled in config)")
		return nil
	}

	log.Printf("Scanning music library in: %s", ms.config.Music.LibraryPath)

	trackCount := 0
	err := filepath.Walk(ms.config.Music.LibraryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if file is a supported audio format
		if ms.extractor.IsAudioFile(path) {
			track, err := ms.extractor.ExtractFromFile(path, 0) // ID will be set by database
			if err != nil {
				log.Printf("Error extracting metadata from %s: %v", path, err)
				return nil // Continue scanning other files
			}

			// Insert track into database
			id, err := ms.db.InsertTrack(track)
			if err != nil {
				log.Printf("Error inserting track into database: %v", err)
				return nil
			}

			trackCount++
			log.Printf("Added track: %s - %s (ID: %d)", track.Artist, track.Title, id)
		}

		return nil
	})

	log.Printf("Scanned %d tracks", trackCount)
	return err
}

// Start starts the music server
func (ms *MusicServer) Start() {
	// Start file watcher if enabled
	if ms.config.Music.WatchForChanges {
		if err := ms.startFileWatcher(); err != nil {
			log.Printf("Warning: Could not start file watcher: %v", err)
		} else {
			defer ms.stopFileWatcher()
		}
	}

	// Set up routes
	ms.setupRoutes()

	// Get track count from database
	tracks, err := ms.db.GetAllTracks()
	trackCount := 0
	if err == nil {
		trackCount = len(tracks)
	}

	localAddress := fmt.Sprintf("http://%s", ms.config.GetAddress())

	log.Printf("🎵 Spotigo server starting on port %s", ms.config.Server.Port)
	log.Printf("📁 Music library contains %d tracks", trackCount)
	if ms.config.Music.WatchForChanges {
		log.Printf("🔍 File watcher monitoring: %s", ms.config.Music.LibraryPath)
	}
	log.Printf("🌐 Local access: %s", localAddress)

	// Start ngrok tunnel if enabled
	if ms.ngrokService != nil {
		ctx := context.Background()
		if err := ms.ngrokService.StartTunnel(ctx, localAddress); err != nil {
			log.Printf("Warning: Could not start ngrok tunnel: %v", err)
		} else {
			defer ms.ngrokService.Stop()
		}
	}

	// Create server with timeout
	server := &http.Server{
		Addr:        ms.config.GetAddress(),
		ReadTimeout: time.Duration(ms.config.Server.ReadTimeout) * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}

func (ms *MusicServer) setupRoutes() {
	http.HandleFunc("/", ms.handleHome)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(ms.config.Server.StaticDir))))
	http.HandleFunc("/api/tracks", ms.handleGetTracks)
	http.HandleFunc("/api/tracks/count", ms.handleGetTrackCount)
	http.HandleFunc("/stream/", ms.handleStreamTrack)

	// Download routes
	http.HandleFunc("/api/download", ms.handleDownloadMusic)
	http.HandleFunc("/api/downloads", ms.handleGetDownloads)
	http.HandleFunc("/api/downloads/", ms.handleGetDownloads) // For specific job ID
	http.HandleFunc("/api/validate-url", ms.handleValidateURL)

	// Playlist routes
	http.HandleFunc("/api/playlists", ms.handleGetPlaylists)
	http.HandleFunc("/api/playlists/create", ms.handleCreatePlaylist)
	http.HandleFunc("/api/playlists/", func(w http.ResponseWriter, r *http.Request) {
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) >= 5 && pathParts[4] == "tracks" {
			if r.Method == "GET" {
				ms.handleGetPlaylistTracks(w, r)
			} else if r.Method == "POST" {
				ms.handleAddTrackToPlaylist(w, r)
			} else if r.Method == "DELETE" {
				ms.handleRemoveTrackFromPlaylist(w, r)
			}
		} else if r.Method == "DELETE" {
			ms.handleDeletePlaylist(w, r)
		}
	})
}
