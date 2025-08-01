package database

import (
	"database/sql"

	"staccato/pkg/models"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	conn *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	db := &Database{conn: conn}
	if err := db.createTables(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *Database) createTables() error {
	// Create tracks table
	tracksTable := `
	CREATE TABLE IF NOT EXISTS tracks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		artist TEXT NOT NULL,
		album TEXT NOT NULL,
		track_number INTEGER DEFAULT 0,
		duration INTEGER DEFAULT 0,
		file_path TEXT NOT NULL UNIQUE,
		file_size INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// Create playlists table
	playlistsTable := `
	CREATE TABLE IF NOT EXISTS playlists (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// Create playlist_tracks junction table
	playlistTracksTable := `
	CREATE TABLE IF NOT EXISTS playlist_tracks (
		playlist_id INTEGER,
		track_id INTEGER,
		position INTEGER,
		added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE,
		FOREIGN KEY (track_id) REFERENCES tracks(id) ON DELETE CASCADE,
		PRIMARY KEY (playlist_id, track_id)
	);`

	// Create indices for better performance
	indices := []string{
		"CREATE INDEX IF NOT EXISTS idx_tracks_artist ON tracks(artist);",
		"CREATE INDEX IF NOT EXISTS idx_tracks_album ON tracks(album);",
		"CREATE INDEX IF NOT EXISTS idx_playlist_tracks_playlist ON playlist_tracks(playlist_id);",
		"CREATE INDEX IF NOT EXISTS idx_playlist_tracks_position ON playlist_tracks(playlist_id, position);",
	}

	tables := []string{tracksTable, playlistsTable, playlistTracksTable}
	for _, table := range tables {
		if _, err := db.conn.Exec(table); err != nil {
			return err
		}
	}

	for _, index := range indices {
		if _, err := db.conn.Exec(index); err != nil {
			return err
		}
	}

	return nil
}

func (db *Database) InsertTrack(track models.Track) (int, error) {
	// Check if track already exists
	var existingID int
	err := db.conn.QueryRow("SELECT id FROM tracks WHERE file_path = ?", track.FilePath).Scan(&existingID)
	if err == nil {
		// Track exists, update it
		_, err = db.conn.Exec(`
			UPDATE tracks SET title = ?, artist = ?, album = ?, track_number = ?, duration = ?, file_size = ?
			WHERE id = ?`,
			track.Title, track.Artist, track.Album, track.TrackNumber, track.Duration, track.FileSize, existingID)
		return existingID, err
	}

	// Insert new track
	result, err := db.conn.Exec(`
		INSERT INTO tracks (title, artist, album, track_number, duration, file_path, file_size)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		track.Title, track.Artist, track.Album, track.TrackNumber, track.Duration, track.FilePath, track.FileSize)

	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	return int(id), err
}

func (db *Database) GetAllTracks() ([]models.Track, error) {
	rows, err := db.conn.Query(`
		SELECT id, title, artist, album, track_number, duration, file_path, file_size
		FROM tracks
		ORDER BY artist, album, track_number, title`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []models.Track
	for rows.Next() {
		var track models.Track
		err := rows.Scan(&track.ID, &track.Title, &track.Artist, &track.Album,
			&track.TrackNumber, &track.Duration, &track.FilePath, &track.FileSize)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

func (db *Database) GetTracksSortedByAlbum() ([]models.Track, error) {
	rows, err := db.conn.Query(`
		SELECT id, title, artist, album, track_number, duration, file_path, file_size
		FROM tracks
		ORDER BY album, track_number, title`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []models.Track
	for rows.Next() {
		var track models.Track
		err := rows.Scan(&track.ID, &track.Title, &track.Artist, &track.Album,
			&track.TrackNumber, &track.Duration, &track.FilePath, &track.FileSize)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

func (db *Database) GetTrackByID(id int) (*models.Track, error) {
	var track models.Track
	err := db.conn.QueryRow(`
		SELECT id, title, artist, album, track_number, duration, file_path, file_size
		FROM tracks WHERE id = ?`, id).Scan(
		&track.ID, &track.Title, &track.Artist, &track.Album,
		&track.TrackNumber, &track.Duration, &track.FilePath, &track.FileSize)

	if err != nil {
		return nil, err
	}
	return &track, nil
}

func (db *Database) CreatePlaylist(name, description string) (int, error) {
	result, err := db.conn.Exec(`
		INSERT INTO playlists (name, description)
		VALUES (?, ?)`, name, description)

	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	return int(id), err
}

func (db *Database) GetAllPlaylists() ([]models.Playlist, error) {
	rows, err := db.conn.Query(`
		SELECT p.id, p.name, p.description, p.created_at,
			   COALESCE(COUNT(pt.track_id), 0) as track_count
		FROM playlists p
		LEFT JOIN playlist_tracks pt ON p.id = pt.playlist_id
		GROUP BY p.id, p.name, p.description, p.created_at
		ORDER BY p.created_at DESC`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var playlists []models.Playlist
	for rows.Next() {
		var playlist models.Playlist
		err := rows.Scan(&playlist.ID, &playlist.Name, &playlist.Description,
			&playlist.CreatedAt, &playlist.TrackCount)
		if err != nil {
			return nil, err
		}
		playlists = append(playlists, playlist)
	}

	return playlists, nil
}

func (db *Database) GetPlaylistTracks(playlistID int) ([]models.Track, error) {
	rows, err := db.conn.Query(`
		SELECT t.id, t.title, t.artist, t.album, t.track_number, t.duration, t.file_path, t.file_size
		FROM tracks t
		JOIN playlist_tracks pt ON t.id = pt.track_id
		WHERE pt.playlist_id = ?
		ORDER BY pt.position`, playlistID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []models.Track
	for rows.Next() {
		var track models.Track
		err := rows.Scan(&track.ID, &track.Title, &track.Artist, &track.Album,
			&track.TrackNumber, &track.Duration, &track.FilePath, &track.FileSize)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

func (db *Database) AddTrackToPlaylist(playlistID, trackID int) error {
	// Get the next position
	var maxPosition sql.NullInt64
	err := db.conn.QueryRow(`
		SELECT MAX(position) FROM playlist_tracks WHERE playlist_id = ?`,
		playlistID).Scan(&maxPosition)

	if err != nil && err != sql.ErrNoRows {
		return err
	}

	position := 1
	if maxPosition.Valid {
		position = int(maxPosition.Int64) + 1
	}

	_, err = db.conn.Exec(`
		INSERT INTO playlist_tracks (playlist_id, track_id, position)
		VALUES (?, ?, ?)
		ON CONFLICT(playlist_id, track_id) DO NOTHING`,
		playlistID, trackID, position)

	return err
}

func (db *Database) RemoveTrackFromPlaylist(playlistID, trackID int) error {
	_, err := db.conn.Exec(`
		DELETE FROM playlist_tracks 
		WHERE playlist_id = ? AND track_id = ?`,
		playlistID, trackID)

	return err
}

func (db *Database) DeletePlaylist(playlistID int) error {
	_, err := db.conn.Exec("DELETE FROM playlists WHERE id = ?", playlistID)
	return err
}

func (db *Database) SearchTracks(query string) ([]models.Track, error) {
	searchQuery := "%" + query + "%"
	rows, err := db.conn.Query(`
		SELECT id, title, artist, album, track_number, duration, file_path, file_size
		FROM tracks
		WHERE title LIKE ? OR artist LIKE ? OR album LIKE ?
		ORDER BY artist, album, track_number, title`,
		searchQuery, searchQuery, searchQuery)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []models.Track
	for rows.Next() {
		var track models.Track
		err := rows.Scan(&track.ID, &track.Title, &track.Artist, &track.Album,
			&track.TrackNumber, &track.Duration, &track.FilePath, &track.FileSize)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

func (db *Database) RemoveTrackByPath(filePath string) error {
	_, err := db.conn.Exec("DELETE FROM tracks WHERE file_path = ?", filePath)
	return err
}

func (db *Database) TrackExists(filePath string) (bool, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM tracks WHERE file_path = ?", filePath).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (db *Database) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}
