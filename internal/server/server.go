package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"staccato/internal/config"
	"staccato/internal/database"
	"staccato/internal/downloader"
	"staccato/internal/metadata"
	"staccato/internal/ngrok"

	"github.com/fsnotify/fsnotify"
)

// MusicServer encapsulates application state and HTTP handling for the music
// service including DB access, metadata extraction, optional downloader,
// optional ngrok tunneling, and filesystem watching.
type MusicServer struct {
	db           *database.Database
	config       *config.Config
	watcher      *fsnotify.Watcher
	extractor    *metadata.Extractor
	downloader   *downloader.Downloader
	ngrokService *ngrok.Service
	server       *http.Server
	handler      http.Handler // root HTTP handler (router + middleware chain)
	shutdownCh   chan struct{}
}

// NewMusicServer constructs a MusicServer with optional components (downloader,
// ngrok). Missing optional components degrade functionality gracefully.
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

	server := &MusicServer{
		db:           db,
		config:       cfg,
		extractor:    metadata.NewExtractor(cfg.Music.SupportedFormats),
		downloader:   dl,
		ngrokService: ngrokSvc,
		shutdownCh:   make(chan struct{}),
	}

	// Attach ingestion capabilities if downloader available
	if server.downloader != nil {
		server.downloader.AttachIngest(server.db, server.extractor)
	}

	return server, nil
}

// ScanMusicLibrary walks the configured music directory ingesting supported
// audio files into the database. Concurrency is sized to runtime.NumCPU.
func (ms *MusicServer) ScanMusicLibrary() error {
	if !ms.config.Music.ScanOnStartup {
		log.Println("Skipping library scan (disabled in config)")
		return nil
	}

	log.Printf("Scanning music library in: %s", ms.config.Music.LibraryPath)

	var wg sync.WaitGroup
	var trackCount int64
	jobs := make(chan string, 100)

	// Start worker pool
	numWorkers := runtime.NumCPU()
	for i := 0; i < numWorkers; i++ {
		go func() {
			for path := range jobs {
				track, err := ms.extractor.ExtractFromFile(path, 0)
				if err != nil {
					log.Printf("Error extracting metadata from %s: %v", path, err)
					wg.Done()
					continue
				}
				_, err = ms.db.InsertTrack(track)
				if err != nil {
					log.Printf("Error inserting track into database: %v", err)
				} else {
					atomic.AddInt64(&trackCount, 1)
					log.Printf("Added track: %s - %s", track.Artist, track.Title)
				}
				wg.Done()
			}
		}()
	}

	// Walk directory and enqueue jobs
	walkErr := filepath.Walk(ms.config.Music.LibraryPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if ms.extractor.IsAudioFile(path) {
			wg.Add(1)
			jobs <- path
		}
		return nil
	})

	// Close jobs channel and wait for all workers
	close(jobs)
	wg.Wait()

	log.Printf("Scanned %d tracks", trackCount)
	return walkErr
}

// Start begins serving HTTP requests and (optionally) establishes an ngrok
// tunnel. It blocks until a shutdown signal is received or a fatal error.
func (ms *MusicServer) Start() {
	// Start file watcher if enabled
	if ms.config.Music.WatchForChanges {
		if err := ms.startFileWatcher(); err != nil {
			log.Printf("Warning: Could not start file watcher: %v", err)
		}
	}

	// Set up routes (build handler chain)
	ms.handler = ms.setupRoutes()

	// Get track count from database
	tracks, err := ms.db.GetAllTracks()
	trackCount := 0
	if err == nil {
		trackCount = len(tracks)
	}

	localAddress := fmt.Sprintf("http://%s", ms.config.GetAddress())

	log.Printf("Staccato server starting on port %s", ms.config.Server.Port)
	log.Printf("Music library contains %d tracks", trackCount)
	if ms.config.Music.WatchForChanges {
		log.Printf("File watcher monitoring: %s", ms.config.Music.LibraryPath)
	}
	log.Printf("Local access: %s", localAddress)

	// Start ngrok tunnel if enabled
	if ms.ngrokService != nil {
		ctx := context.Background()
		if err := ms.ngrokService.StartTunnel(ctx, localAddress); err != nil {
			log.Printf("Warning: Could not start ngrok tunnel: %v", err)
		}
	}

	// Create server with proper timeouts
	ms.server = &http.Server{
		Addr:         ms.config.GetAddress(),
		ReadTimeout:  time.Duration(ms.config.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(ms.config.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(ms.config.Server.IdleTimeout) * time.Second,
		Handler:      ms.handler,
	}

	// Start server in a goroutine
	serverErrCh := make(chan error, 1)
	go func() {
		log.Printf("HTTP server listening on %s", ms.server.Addr)
		if err := ms.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- fmt.Errorf("server failed to start: %w", err)
		}
	}()

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErrCh:
		log.Fatalf("Server error: %v", err)
	case <-ms.shutdownCh:
		log.Println("Shutdown signal received, starting graceful shutdown...")
	}
}

func (ms *MusicServer) setupRoutes() http.Handler {
	// Create a new ServeMux
	mux := http.NewServeMux()

	// Set up routes
	mux.HandleFunc("/", ms.handleHome)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(ms.config.Server.StaticDir))))
	mux.HandleFunc("/api/tracks", ms.handleGetTracks)
	mux.HandleFunc("/api/tracks/count", ms.handleGetTrackCount)
	mux.HandleFunc("/stream/", ms.handleStreamTrack)
	mux.HandleFunc("/albumart/", ms.handleAlbumArt) // Album art endpoint
	mux.HandleFunc("/health", ms.handleHealthCheck) // Health check endpoint

	// Download routes
	mux.HandleFunc("/api/download", ms.handleDownloadMusic)
	mux.HandleFunc("/api/downloads", ms.handleGetDownloads)
	mux.HandleFunc("/api/downloads/", ms.handleGetDownloads) // For specific job ID
	mux.HandleFunc("/api/validate-url", ms.handleValidateURL)
	mux.HandleFunc("/api/downloads/cleanup", ms.handleCleanupDownloads)

	// Playlist routes
	mux.HandleFunc("/api/playlists", ms.handleGetPlaylists)
	mux.HandleFunc("/api/playlists/create", ms.handleCreatePlaylist)
	mux.HandleFunc("/api/playlists/", func(w http.ResponseWriter, r *http.Request) {
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) >= 5 && pathParts[4] == "tracks" {
			switch r.Method {
			case "GET":
				ms.handleGetPlaylistTracks(w, r)
			case "POST":
				ms.handleAddTrackToPlaylist(w, r)
			case "DELETE":
				ms.handleRemoveTrackFromPlaylist(w, r)
			}
		} else {
			switch r.Method {
			case "DELETE":
				ms.handleDeletePlaylist(w, r)
			case "PUT":
				ms.handleUpdatePlaylist(w, r)
			}
		}
	})

	// Apply middleware chain (order: panic recovery -> logging)
	handler := ms.panicRecoveryMiddleware(mux)
	handler = ms.requestLoggingMiddleware(handler)
	return handler
}

// Shutdown gracefully stops all server components (HTTP listener, watcher,
// ngrok tunnel, database connection).
func (ms *MusicServer) Shutdown() {
	log.Println("Shutting down music server...")

	// Signal the start method to begin shutdown (safely)
	select {
	case <-ms.shutdownCh:
		// Already signaled
	default:
		close(ms.shutdownCh)
	}

	// Create a context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop the HTTP server gracefully
	if ms.server != nil {
		log.Println("Shutting down HTTP server...")
		if err := ms.server.Shutdown(ctx); err != nil {
			log.Printf("Error during HTTP server shutdown: %v", err)
			// Force close if graceful shutdown fails
			if err := ms.server.Close(); err != nil {
				log.Printf("Error force closing HTTP server: %v", err)
			}
		} else {
			log.Println("HTTP server shut down gracefully")
		}
	}

	// Stop file watcher
	log.Println("Stopping file watcher...")
	ms.stopFileWatcher()

	// Stop ngrok service
	if ms.ngrokService != nil {
		log.Println("Stopping ngrok service...")
		ms.ngrokService.Stop()
	}

	// Close database connection
	if ms.db != nil {
		log.Println("Closing database connection...")
		ms.db.Close()
	}

	log.Println("Music server shutdown complete")
}

// RequestShutdown can be called from other goroutines to initiate graceful
// shutdown (idempotent).
func (ms *MusicServer) RequestShutdown() {
	select {
	case <-ms.shutdownCh:
		// Already shutting down
		return
	default:
		close(ms.shutdownCh)
	}
}
