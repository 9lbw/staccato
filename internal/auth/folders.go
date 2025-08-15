package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UserFolderManager handles user-specific folder operations
type UserFolderManager struct {
	enabled  bool
	basePath string
}

// NewUserFolderManager creates a new user folder manager
func NewUserFolderManager(enabled bool, basePath string) *UserFolderManager {
	return &UserFolderManager{
		enabled:  enabled,
		basePath: basePath,
	}
}

// IsEnabled returns whether user folders are enabled
func (ufm *UserFolderManager) IsEnabled() bool {
	return ufm.enabled
}

// CreateUserFolder creates a folder for a new user
func (ufm *UserFolderManager) CreateUserFolder(username string) error {
	if !ufm.enabled {
		return nil // No-op if user folders are disabled
	}

	// Sanitize username for folder name
	folderName := sanitizeUsername(username)
	userPath := filepath.Join(ufm.basePath, folderName)

	// Create the user's music directory
	if err := os.MkdirAll(userPath, 0755); err != nil {
		return fmt.Errorf("failed to create user folder: %w", err)
	}

	// Create a README file with instructions
	readmePath := filepath.Join(userPath, "README.txt")
	readmeContent := fmt.Sprintf(`This is %s's personal music folder.

Upload your music files here and they will be automatically
scanned and added to your personal library.

Supported formats: .flac, .mp3, .wav, .m4a

The server will monitor this folder for changes and automatically
add new music to your library.
`, username)

	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		// Don't fail if we can't create README, just log it
		// We'll add logging later if needed
	}

	return nil
}

// GetUserMusicPath returns the music path for a user
func (ufm *UserFolderManager) GetUserMusicPath(username string) string {
	if !ufm.enabled {
		return "" // Return empty if user folders are disabled
	}

	folderName := sanitizeUsername(username)
	return filepath.Join(ufm.basePath, folderName)
}

// GetAllUserPaths returns all user music paths for scanning
func (ufm *UserFolderManager) GetAllUserPaths() ([]string, error) {
	if !ufm.enabled {
		return nil, nil
	}

	// Check if base path exists
	if _, err := os.Stat(ufm.basePath); os.IsNotExist(err) {
		return nil, nil // Return empty if base path doesn't exist
	}

	entries, err := os.ReadDir(ufm.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read user base directory: %w", err)
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			userPath := filepath.Join(ufm.basePath, entry.Name())
			paths = append(paths, userPath)
		}
	}

	return paths, nil
}

// GetOwnerFromPath determines the username (owner) from a file path within user folders
func (ufm *UserFolderManager) GetOwnerFromPath(filePath string) string {
	if !ufm.enabled {
		return "" // No owner if user folders disabled
	}

	// Make the paths absolute for comparison
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return ""
	}

	absBasePath, err := filepath.Abs(ufm.basePath)
	if err != nil {
		return ""
	}

	// Check if the file is within the user base path
	relPath, err := filepath.Rel(absBasePath, absFilePath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "" // File is outside user base path
	}

	// Extract the first directory component as the user folder
	pathParts := strings.Split(relPath, string(filepath.Separator))
	if len(pathParts) > 0 && pathParts[0] != "" {
		// This is the sanitized username (folder name)
		return pathParts[0]
	}

	return ""
}

// DeleteUserFolder removes a user's folder (if user is deleted)
func (ufm *UserFolderManager) DeleteUserFolder(username string) error {
	if !ufm.enabled {
		return nil
	}

	folderName := sanitizeUsername(username)
	userPath := filepath.Join(ufm.basePath, folderName)

	if _, err := os.Stat(userPath); os.IsNotExist(err) {
		return nil // Folder doesn't exist, nothing to do
	}

	return os.RemoveAll(userPath)
}

// sanitizeUsername removes unsafe characters from username for folder name
func sanitizeUsername(username string) string {
	// Replace unsafe characters with underscores
	unsafe := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", " "}
	result := username

	for _, char := range unsafe {
		result = strings.ReplaceAll(result, char, "_")
	}

	// Remove any leading/trailing dots or spaces
	result = strings.Trim(result, ". ")

	// Ensure it's not empty
	if result == "" {
		result = "user"
	}

	return result
}
