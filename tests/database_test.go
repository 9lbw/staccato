package tests

import (
	"path/filepath"
	"testing"

	"staccato/internal/database"
	"staccato/pkg/models"
)

func TestDatabase(t *testing.T) {
	// Create test database
	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "test.db")

	db, err := database.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	t.Run("InsertAndGetTrack", func(t *testing.T) {
		track := models.Track{
			Title:       "Test Song",
			Artist:      "Test Artist",
			Album:       "Test Album",
			TrackNumber: 1,
			Duration:    180,
			FilePath:    "/test/song.mp3",
			FileSize:    1024000,
			Owner:       "testuser",
		}

		// Insert track
		id, err := db.InsertTrack(track)
		if err != nil {
			t.Fatalf("Failed to insert track: %v", err)
		}

		// Get track by ID
		retrievedTrack, err := db.GetTrackByID(id)
		if err != nil {
			t.Fatalf("Failed to get track by ID: %v", err)
		}

		// Verify track data
		if retrievedTrack.Title != track.Title {
			t.Errorf("Expected title %s, got %s", track.Title, retrievedTrack.Title)
		}
		if retrievedTrack.Artist != track.Artist {
			t.Errorf("Expected artist %s, got %s", track.Artist, retrievedTrack.Artist)
		}
		if retrievedTrack.Owner != track.Owner {
			t.Errorf("Expected owner %s, got %s", track.Owner, retrievedTrack.Owner)
		}
	})

	t.Run("GetAllTracks", func(t *testing.T) {
		tracks, err := db.GetAllTracks()
		if err != nil {
			t.Fatalf("Failed to get all tracks: %v", err)
		}

		if len(tracks) == 0 {
			t.Error("Expected at least one track")
		}
	})

	t.Run("SearchTracks", func(t *testing.T) {
		// Search for the test track
		tracks, err := db.SearchTracks("Test")
		if err != nil {
			t.Fatalf("Failed to search tracks: %v", err)
		}

		if len(tracks) == 0 {
			t.Error("Expected to find tracks with 'Test'")
		}
	})

	t.Run("TracksByOwner", func(t *testing.T) {
		tracks, err := db.GetTracksByOwner("testuser")
		if err != nil {
			t.Fatalf("Failed to get tracks by owner: %v", err)
		}

		if len(tracks) == 0 {
			t.Error("Expected to find tracks for testuser")
		}

		// Verify all tracks belong to testuser
		for _, track := range tracks {
			if track.Owner != "testuser" {
				t.Errorf("Expected owner testuser, got %s", track.Owner)
			}
		}
	})

	t.Run("UpdateTrack", func(t *testing.T) {
		// Create a new track
		track := models.Track{
			Title:    "Original Title",
			Artist:   "Original Artist",
			Album:    "Original Album",
			FilePath: "/test/update.mp3",
			FileSize: 500000,
			Owner:    "testuser",
		}

		id, err := db.InsertTrack(track)
		if err != nil {
			t.Fatalf("Failed to insert track for update test: %v", err)
		}

		// Update the same track (same file path)
		updatedTrack := track
		updatedTrack.Title = "Updated Title"
		updatedTrack.Artist = "Updated Artist"

		updatedID, err := db.InsertTrack(updatedTrack)
		if err != nil {
			t.Fatalf("Failed to update track: %v", err)
		}

		// Should return the same ID
		if updatedID != id {
			t.Errorf("Expected same ID %d, got %d", id, updatedID)
		}

		// Verify the track was updated
		retrievedTrack, err := db.GetTrackByID(id)
		if err != nil {
			t.Fatalf("Failed to get updated track: %v", err)
		}

		if retrievedTrack.Title != "Updated Title" {
			t.Errorf("Expected updated title, got %s", retrievedTrack.Title)
		}
	})

	t.Run("TrackExists", func(t *testing.T) {
		exists, err := db.TrackExists("/test/song.mp3")
		if err != nil {
			t.Fatalf("Failed to check if track exists: %v", err)
		}

		if !exists {
			t.Error("Expected track to exist")
		}

		exists, err = db.TrackExists("/nonexistent/track.mp3")
		if err != nil {
			t.Fatalf("Failed to check if nonexistent track exists: %v", err)
		}

		if exists {
			t.Error("Expected track to not exist")
		}
	})

	t.Run("DeleteTracksByOwner", func(t *testing.T) {
		// Add another track for deletion test
		track := models.Track{
			Title:    "Delete Me",
			Artist:   "Delete Artist",
			Album:    "Delete Album",
			FilePath: "/test/delete.mp3",
			FileSize: 300000,
			Owner:    "deleteuser",
		}

		_, err := db.InsertTrack(track)
		if err != nil {
			t.Fatalf("Failed to insert track for deletion test: %v", err)
		}

		// Verify track exists
		tracks, err := db.GetTracksByOwner("deleteuser")
		if err != nil {
			t.Fatalf("Failed to get tracks for deleteuser: %v", err)
		}
		if len(tracks) == 0 {
			t.Fatal("Expected track for deleteuser")
		}

		// Delete tracks by owner
		err = db.DeleteTracksByOwner("deleteuser")
		if err != nil {
			t.Fatalf("Failed to delete tracks by owner: %v", err)
		}

		// Verify tracks are deleted
		tracks, err = db.GetTracksByOwner("deleteuser")
		if err != nil {
			t.Fatalf("Failed to get tracks after deletion: %v", err)
		}
		if len(tracks) != 0 {
			t.Errorf("Expected 0 tracks for deleted user, got %d", len(tracks))
		}
	})

	t.Run("RemoveTrackByPath", func(t *testing.T) {
		// Remove the original test track
		err := db.RemoveTrackByPath("/test/song.mp3")
		if err != nil {
			t.Fatalf("Failed to remove track by path: %v", err)
		}

		// Verify track is removed
		exists, err := db.TrackExists("/test/song.mp3")
		if err != nil {
			t.Fatalf("Failed to check if removed track exists: %v", err)
		}

		if exists {
			t.Error("Expected track to be removed")
		}
	})
}

func TestDatabasePlaylists(t *testing.T) {
	// Create test database
	testDir := t.TempDir()
	dbPath := filepath.Join(testDir, "test_playlists.db")

	db, err := database.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Insert some test tracks first
	track1 := models.Track{
		Title:    "Track 1",
		Artist:   "Artist 1",
		Album:    "Album 1",
		FilePath: "/test/track1.mp3",
		FileSize: 1000000,
	}
	track2 := models.Track{
		Title:    "Track 2",
		Artist:   "Artist 2",
		Album:    "Album 2",
		FilePath: "/test/track2.mp3",
		FileSize: 2000000,
	}

	trackID1, err := db.InsertTrack(track1)
	if err != nil {
		t.Fatalf("Failed to insert test track 1: %v", err)
	}

	trackID2, err := db.InsertTrack(track2)
	if err != nil {
		t.Fatalf("Failed to insert test track 2: %v", err)
	}

	t.Run("CreatePlaylist", func(t *testing.T) {
		playlistID, err := db.CreatePlaylist("Test Playlist", "A test playlist")
		if err != nil {
			t.Fatalf("Failed to create playlist: %v", err)
		}

		if playlistID <= 0 {
			t.Error("Expected valid playlist ID")
		}
	})

	t.Run("GetAllPlaylists", func(t *testing.T) {
		playlists, err := db.GetAllPlaylists()
		if err != nil {
			t.Fatalf("Failed to get all playlists: %v", err)
		}

		if len(playlists) == 0 {
			t.Error("Expected at least one playlist")
		}
	})

	t.Run("PlaylistOperations", func(t *testing.T) {
		// Create a playlist for testing operations
		playlistID, err := db.CreatePlaylist("Operations Test", "Testing playlist operations")
		if err != nil {
			t.Fatalf("Failed to create test playlist: %v", err)
		}

		// Add tracks to playlist
		err = db.AddTrackToPlaylist(playlistID, trackID1)
		if err != nil {
			t.Fatalf("Failed to add track 1 to playlist: %v", err)
		}

		err = db.AddTrackToPlaylist(playlistID, trackID2)
		if err != nil {
			t.Fatalf("Failed to add track 2 to playlist: %v", err)
		}

		// Get playlist tracks
		tracks, err := db.GetPlaylistTracks(playlistID)
		if err != nil {
			t.Fatalf("Failed to get playlist tracks: %v", err)
		}

		if len(tracks) != 2 {
			t.Errorf("Expected 2 tracks in playlist, got %d", len(tracks))
		}

		// Remove a track from playlist
		err = db.RemoveTrackFromPlaylist(playlistID, trackID1)
		if err != nil {
			t.Fatalf("Failed to remove track from playlist: %v", err)
		}

		// Verify track was removed
		tracks, err = db.GetPlaylistTracks(playlistID)
		if err != nil {
			t.Fatalf("Failed to get playlist tracks after removal: %v", err)
		}

		if len(tracks) != 1 {
			t.Errorf("Expected 1 track in playlist after removal, got %d", len(tracks))
		}

		// Update playlist
		err = db.UpdatePlaylist(playlistID, "Updated Name", "Updated description", "/path/to/cover.jpg")
		if err != nil {
			t.Fatalf("Failed to update playlist: %v", err)
		}

		// Delete playlist
		err = db.DeletePlaylist(playlistID)
		if err != nil {
			t.Fatalf("Failed to delete playlist: %v", err)
		}

		// Verify playlist is deleted (GetPlaylistTracks should return empty)
		tracks, err = db.GetPlaylistTracks(playlistID)
		if err != nil {
			t.Fatalf("Failed to get tracks from deleted playlist: %v", err)
		}

		if len(tracks) != 0 {
			t.Errorf("Expected 0 tracks from deleted playlist, got %d", len(tracks))
		}
	})
}
