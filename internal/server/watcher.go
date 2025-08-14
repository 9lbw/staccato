package server

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// startFileWatcher initializes fsnotify watcher for recursive music dir monitoring.
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

	ms.logger.WithField("library_path", ms.config.Music.LibraryPath).Info("File watcher started")
	return nil
}

// addDirectoryToWatcher recursively walks and adds subdirectories to watcher.
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

// watchFiles selects on watcher channels and dispatches events.
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
			ms.logger.WithError(err).Error("File watcher error")
		}
	}
}

// handleFileEvent applies filtering & delegates creation/removal actions.
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
			ms.logger.WithField("directory", event.Name).Info("Watching new directory")
		}
	}
}

// handleNewFile extracts metadata & inserts new track if unseen.
func (ms *MusicServer) handleNewFile(filePath string) {
	ms.logger.WithField("file_path", filePath).Info("New audio file detected")

	// Check if file already exists in database
	exists, err := ms.db.TrackExists(filePath)
	if err != nil {
		ms.logger.WithError(err).WithField("file_path", filePath).Error("Error checking if track exists")
		return
	}
	if exists {
		ms.logger.WithField("file_path", filePath).Debug("Track already exists in database")
		return
	}

	// Extract metadata and add to database
	track, err := ms.extractor.ExtractFromFile(filePath, 0)
	if err != nil {
		ms.logger.WithError(err).WithField("file_path", filePath).Error("Error extracting metadata")
		return
	}

	id, err := ms.db.InsertTrack(track)
	if err != nil {
		ms.logger.WithError(err).Error("Error inserting new track into database")
		return
	}

	ms.logger.WithFields(logrus.Fields{
		"artist": track.Artist,
		"title":  track.Title,
		"album":  track.Album,
		"id":     id,
	}).Info("Added new track")
}

// handleRemovedFile removes track rows referencing deleted audio files.
func (ms *MusicServer) handleRemovedFile(filePath string) {
	ms.logger.WithField("file_path", filePath).Info("Audio file removed")

	err := ms.db.RemoveTrackByPath(filePath)
	if err != nil {
		ms.logger.WithError(err).WithField("file_path", filePath).Error("Error removing track from database")
		return
	}

	ms.logger.WithField("file_path", filePath).Info("Removed track from database")
}

// stopFileWatcher closes the watcher (idempotent).
func (ms *MusicServer) stopFileWatcher() {
	if ms.watcher != nil {
		ms.watcher.Close()
	}
}
