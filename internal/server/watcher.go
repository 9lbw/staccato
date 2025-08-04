package server

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// startFileWatcher starts monitoring the music directory for changes
func (ms *MusicServer) startFileWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	ms.watcher = watcher

	// Start monitoring in a goroutine
	go ms.watchFiles()

	// Add the music directory to the watcher
	err = ms.addDirectoryToWatcher(ms.config.Music.LibraryPath)
	if err != nil {
		return err
	}

	log.Printf("File watcher started for: %s", ms.config.Music.LibraryPath)
	return nil
}

// addDirectoryToWatcher recursively adds directories to the file watcher
func (ms *MusicServer) addDirectoryToWatcher(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return ms.watcher.Add(path)
		}
		return nil
	})
}

// watchFiles monitors file system events
func (ms *MusicServer) watchFiles() {
	defer ms.watcher.Close()

	for {
		select {
		case event, ok := <-ms.watcher.Events:
			if !ok {
				return
			}
			ms.handleFileEvent(event)

		case err, ok := <-ms.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

// handleFileEvent processes file system events
func (ms *MusicServer) handleFileEvent(event fsnotify.Event) {
	// Ignore temporary files and hidden files
	fileName := filepath.Base(event.Name)
	if strings.HasPrefix(fileName, ".") || strings.HasSuffix(fileName, ".tmp") {
		return
	}

	isAudioFile := ms.extractor.IsAudioFile(event.Name)

	switch {
	case event.Has(fsnotify.Create) && isAudioFile:
		// Dispatch new file processing asynchronously
		go func(name string) {
			time.Sleep(500 * time.Millisecond) // Ensure file is fully written
			ms.handleNewFile(name)
		}(event.Name)

	case event.Has(fsnotify.Remove) && isAudioFile:
		// Dispatch removal processing asynchronously
		go ms.handleRemovedFile(event.Name)

	case event.Has(fsnotify.Create):
		// Check if it's a new directory
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			ms.watcher.Add(event.Name)
			log.Printf("Watching new directory: %s", event.Name)
		}
	}
}

// handleNewFile processes newly added audio files
func (ms *MusicServer) handleNewFile(filePath string) {
	log.Printf("New audio file detected: %s", filePath)

	// Check if file already exists in database
	exists, err := ms.db.TrackExists(filePath)
	if err != nil {
		log.Printf("Error checking if track exists: %v", err)
		return
	}
	if exists {
		log.Printf("Track already exists in database: %s", filePath)
		return
	}

	// Extract metadata and add to database
	track, err := ms.extractor.ExtractFromFile(filePath, 0)
	if err != nil {
		log.Printf("Error extracting metadata from %s: %v", filePath, err)
		return
	}

	id, err := ms.db.InsertTrack(track)
	if err != nil {
		log.Printf("Error inserting new track into database: %v", err)
		return
	}

	log.Printf("Added new track: %s - %s (ID: %d)", track.Artist, track.Title, id)
}

// handleRemovedFile processes removed audio files
func (ms *MusicServer) handleRemovedFile(filePath string) {
	log.Printf("Audio file removed: %s", filePath)

	err := ms.db.RemoveTrackByPath(filePath)
	if err != nil {
		log.Printf("Error removing track from database: %v", err)
		return
	}

	log.Printf("Removed track from database: %s", filePath)
}

// stopFileWatcher stops the file watcher
func (ms *MusicServer) stopFileWatcher() {
	if ms.watcher != nil {
		ms.watcher.Close()
	}
}
