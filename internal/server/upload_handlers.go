package server

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// handleUploadTrack handles file uploads to user's music folder
func (ms *MusicServer) handleUploadTrack(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		ms.respondWithError(w, r, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	// Check if uploads are enabled
	if !ms.config.Auth.AllowUploads {
		ms.respondWithError(w, r, http.StatusForbidden, "File uploads are disabled", nil)
		return
	}

	// Check if user folders are enabled (required for uploads)
	if !ms.authService.GetUserFolderManager().IsEnabled() {
		ms.respondWithError(w, r, http.StatusBadRequest, "User folders must be enabled for uploads", nil)
		return
	}

	// Get current user from context
	userInterface := r.Context().Value(UserContextKey)
	if userInterface == nil {
		ms.respondWithError(w, r, http.StatusUnauthorized, "Authentication required", nil)
		return
	}
	username := userInterface.(string)

	// Parse multipart form with size limit
	maxSize := ms.config.Auth.MaxUploadSize * 1024 * 1024 // Convert MB to bytes
	err := r.ParseMultipartForm(maxSize)
	if err != nil {
		ms.respondWithError(w, r, http.StatusBadRequest, "Failed to parse upload form", err)
		return
	}

	// Get the uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		ms.respondWithError(w, r, http.StatusBadRequest, "No file provided", err)
		return
	}
	defer file.Close()

	// Validate file extension
	filename := header.Filename
	if !ms.isValidAudioFile(filename) {
		ms.respondWithError(w, r, http.StatusBadRequest, "Invalid file type. Supported formats: "+strings.Join(ms.config.Music.SupportedFormats, ", "), nil)
		return
	}

	// Get user's music folder
	userMusicPath := ms.authService.GetUserFolderManager().GetUserMusicPath(username)
	if userMusicPath == "" {
		ms.respondWithError(w, r, http.StatusInternalServerError, "Failed to determine user music path", nil)
		return
	}

	// Create user folder if it doesn't exist
	if err := os.MkdirAll(userMusicPath, 0755); err != nil {
		ms.respondWithError(w, r, http.StatusInternalServerError, "Failed to create user folder", err)
		return
	}

	// Sanitize filename to prevent path traversal
	safeFilename := filepath.Base(filename)
	if safeFilename == "." || safeFilename == "/" {
		safeFilename = "uploaded_file" + filepath.Ext(filename)
	}

	// Check if file already exists and create unique name if needed
	destPath := filepath.Join(userMusicPath, safeFilename)
	counter := 1
	for {
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			break
		}
		// File exists, try with counter
		ext := filepath.Ext(safeFilename)
		nameWithoutExt := strings.TrimSuffix(safeFilename, ext)
		destPath = filepath.Join(userMusicPath, fmt.Sprintf("%s_%d%s", nameWithoutExt, counter, ext))
		counter++
	}

	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		ms.respondWithError(w, r, http.StatusInternalServerError, "Failed to create destination file", err)
		return
	}
	defer destFile.Close()

	// Copy file content
	_, err = io.Copy(destFile, file)
	if err != nil {
		os.Remove(destPath) // Clean up on error
		ms.respondWithError(w, r, http.StatusInternalServerError, "Failed to save file", err)
		return
	}

	// Extract metadata and add to database
	track, err := ms.extractor.ExtractFromFile(destPath, 0)
	if err != nil {
		ms.logger.WithError(err).WithField("file_path", destPath).Warn("Failed to extract metadata from uploaded file")
		// Don't fail the upload, just log the warning
	} else {
		// Set ownership
		track.Owner = username

		// Insert into database
		trackID, err := ms.db.InsertTrack(track)
		if err != nil {
			ms.logger.WithError(err).WithField("file_path", destPath).Error("Failed to insert uploaded track into database")
		} else {
			ms.logger.WithFields(logrus.Fields{
				"username": username,
				"filename": safeFilename,
				"track_id": trackID,
				"artist":   track.Artist,
				"title":    track.Title,
			}).Info("File uploaded and added to library")
		}
	}

	// Return success response
	response := map[string]interface{}{
		"success":  true,
		"message":  "File uploaded successfully",
		"filename": filepath.Base(destPath),
	}
	ms.respondJSON(w, response)
}

// isValidAudioFile checks if the filename has a supported audio extension
func (ms *MusicServer) isValidAudioFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, supportedExt := range ms.config.Music.SupportedFormats {
		if ext == supportedExt {
			return true
		}
	}
	return false
}
