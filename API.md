# API Documentation for Staccato

## Overview
Staccato is a self-hosted music streaming server built in Go that provides a RESTful API for managing and streaming audio files. The server supports music library management, playlist creation, and audio downloading from external sources via yt-dlp.

## Base URL
```
http://localhost:8000
```
*Note: The default port is 8000, but this can be configured via the `config.toml` file.*

## Authentication
Currently, no authentication is required for API access. All endpoints are publicly accessible.

## Global Headers
All API endpoints support CORS when enabled in configuration:
- **CORS:** Configurable via `enable_cors` setting in server configuration
- **Content-Type:** `application/json` for all API responses (unless specified otherwise)

## Endpoints

### Health & System

#### GET /health
**Description:** Health check endpoint providing server status and basic statistics

**Authentication:** Not Required

**Request:**
- **Headers:** None required

**Response:**

*Success (200 OK):*
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T12:00:00Z",
  "database": "healthy",
  "storage": "healthy",
  "activeSessions": 0,
  "trackCount": 1234,
  "details": {}
}
```

*Unhealthy (503 Service Unavailable):*
```json
{
  "status": "unhealthy",
  "timestamp": "2024-01-01T12:00:00Z",
  "database": "error",
  "storage": "healthy",
  "activeSessions": 0,
  "trackCount": 0,
  "details": {
    "error": "Database connection failed"
  }
}
```

**Client Implementation Notes:**
- Use this endpoint for monitoring server availability
- Check `status` field for overall health status
- Monitor `trackCount` for library scanning progress

---

### Music Library

#### GET /api/tracks
**Description:** Retrieve all tracks in the music library with optional filtering and sorting

**Authentication:** Not Required

**Request:**
- **Query Parameters:**
  - `search` (string, optional): Search term to filter tracks by title, artist, or album
  - `sort` (string, optional): Sort method - use "album" to sort by album, otherwise defaults to artist/album/track order

**Response:**

*Success (200 OK):*
```json
[
  {
    "id": 1,
    "title": "Track Title",
    "artist": "Artist Name",
    "album": "Album Name",
    "trackNumber": 1,
    "duration": 240,
    "fileSize": 8388608,
    "hasAlbumArt": true,
    "albumArtId": "abc123"
  }
]
```

*Error (500 Internal Server Error):*
```json
"Error retrieving tracks"
```

**Client Implementation Notes:**
- The `filePath` field is intentionally excluded from responses for security
- Use `albumArtId` with `/albumart/` endpoint to display album artwork
- Duration is provided in seconds
- Search is case-insensitive and searches across title, artist, and album fields

---

#### GET /api/tracks/count
**Description:** Get the total number of tracks in the music library

**Authentication:** Not Required

**Request:**
- **Headers:** None required

**Response:**

*Success (200 OK):*
```json
{
  "count": 1234
}
```

**Client Implementation Notes:**
- Useful for displaying library statistics
- Count reflects actual database entries, not file system scan

---

#### GET /stream/{trackId}
**Description:** Stream audio file for a specific track with support for HTTP range requests

**Authentication:** Not Required

**Request:**
- **Path Parameters:**
  - `trackId` (integer, required): Unique identifier of the track to stream
- **Headers:**
  - `Range` (string, optional): HTTP range header for partial content requests (e.g., "bytes=0-1023")

**Response:**

*Success (200 OK) - Full Content:*
- **Content-Type:** `audio/mpeg`, `audio/flac`, `audio/wav`, or `audio/mp4` (based on file type)
- **Content-Length:** File size in bytes
- **Accept-Ranges:** `bytes`
- **Body:** Binary audio data

*Success (206 Partial Content) - Range Request:*
- **Content-Type:** Audio MIME type
- **Content-Range:** `bytes {start}-{end}/{total}`
- **Content-Length:** Range size in bytes
- **Body:** Requested portion of audio data

*Error (400 Bad Request):*
```
"Invalid track ID"
```

*Error (404 Not Found):*
```
"Track not found"
```

*Error (500 Internal Server Error):*
```
"Error opening audio file"
```

**Client Implementation Notes:**
- Supports HTTP range requests for seeking functionality
- Use range requests for progressive loading and seeking
- Content-Type header indicates the audio format
- Direct streaming URL can be used in HTML5 audio elements

---

#### GET /albumart/{albumArtId}
**Description:** Retrieve album artwork image data

**Authentication:** Not Required

**Request:**
- **Path Parameters:**
  - `albumArtId` (string, required): Album art identifier from track data

**Response:**

*Success (200 OK):*
- **Content-Type:** `image/jpeg`, `image/png`, or appropriate image MIME type
- **Cache-Control:** `public, max-age=3600`
- **Body:** Binary image data

*Error (400 Bad Request):*
```
"Invalid album art ID"
```

*Error (404 Not Found):*
```
"Album art not found"
```

**Client Implementation Notes:**
- Images are cached for 1 hour by default
- Use the `albumArtId` from track objects to construct requests
- Handle 404 errors gracefully by hiding album art or showing placeholders

---

### Playlists

#### GET /api/playlists
**Description:** Retrieve all user-created playlists with track counts

**Authentication:** Not Required

**Request:**
- **Headers:** None required

**Response:**

*Success (200 OK):*
```json
[
  {
    "id": 1,
    "name": "My Playlist",
    "description": "A collection of favorite songs",
    "coverPath": "/path/to/cover.jpg",
    "createdAt": "2024-01-01T12:00:00Z",
    "trackCount": 25
  }
]
```

**Client Implementation Notes:**
- `description` and `coverPath` may be empty strings
- `trackCount` is calculated from playlist_tracks relationships
- Results are ordered by creation date (newest first)

---

#### POST /api/playlists/create
**Description:** Create a new playlist

**Authentication:** Not Required

**Request:**
- **Headers:**
  - `Content-Type: application/json`
- **Request Body:**
```json
{
  "name": "My New Playlist",
  "description": "Optional description"
}
```

**Response:**

*Success (200 OK):*
```json
{
  "id": 1,
  "message": "Playlist created successfully"
}
```

*Error (400 Bad Request):*
```
"Playlist name is required"
```

*Error (500 Internal Server Error):*
```
"Error creating playlist"
```

**Client Implementation Notes:**
- `name` field is required and cannot be empty
- `description` is optional
- Returns the new playlist ID for immediate use

---

#### GET /api/playlists/{playlistId}/tracks
**Description:** Get all tracks in a specific playlist

**Authentication:** Not Required

**Request:**
- **Path Parameters:**
  - `playlistId` (integer, required): ID of the playlist

**Response:**

*Success (200 OK):*
```json
[
  {
    "id": 1,
    "title": "Track Title",
    "artist": "Artist Name",
    "album": "Album Name",
    "trackNumber": 1,
    "duration": 240,
    "fileSize": 8388608,
    "hasAlbumArt": true,
    "albumArtId": "abc123"
  }
]
```

*Error (400 Bad Request):*
```
"Invalid playlist ID"
```

*Error (500 Internal Server Error):*
```
"Error retrieving playlist tracks"
```

**Client Implementation Notes:**
- Tracks are returned in playlist order (by position)
- Returns empty array for playlists with no tracks
- Track format matches the main tracks endpoint

---

#### POST /api/playlists/{playlistId}/tracks
**Description:** Add a track to a playlist

**Authentication:** Not Required

**Request:**
- **Path Parameters:**
  - `playlistId` (integer, required): ID of the playlist
- **Headers:**
  - `Content-Type: application/json`
- **Request Body:**
```json
{
  "trackId": 1
}
```

**Response:**

*Success (200 OK):*
```json
{
  "message": "Track added to playlist"
}
```

*Error (400 Bad Request):*
```
"Invalid playlist ID" or "Invalid JSON"
```

*Error (500 Internal Server Error):*
```
"Error adding track to playlist"
```

**Client Implementation Notes:**
- Duplicate tracks are ignored (ON CONFLICT DO NOTHING)
- Track position is automatically assigned as the last position + 1
- Validate that both playlist and track exist before calling

---

#### DELETE /api/playlists/{playlistId}/tracks
**Description:** Remove a track from a playlist

**Authentication:** Not Required

**Request:**
- **Path Parameters:**
  - `playlistId` (integer, required): ID of the playlist
- **Headers:**
  - `Content-Type: application/json`
- **Request Body:**
```json
{
  "trackId": 1
}
```

**Response:**

*Success (200 OK):*
```json
{
  "message": "Track removed from playlist"
}
```

*Error (400 Bad Request):*
```
"Invalid playlist ID" or "Invalid JSON"
```

*Error (500 Internal Server Error):*
```
"Error removing track from playlist"
```

**Client Implementation Notes:**
- Removing non-existent track-playlist combinations succeeds silently
- No automatic position reordering occurs

---

#### DELETE /api/playlists/{playlistId}
**Description:** Delete a playlist and all its track associations

**Authentication:** Not Required

**Request:**
- **Path Parameters:**
  - `playlistId` (integer, required): ID of the playlist to delete

**Response:**

*Success (200 OK):*
```json
{
  "message": "Playlist deleted successfully"
}
```

*Error (400 Bad Request):*
```
"Invalid playlist ID"
```

*Error (500 Internal Server Error):*
```
"Error deleting playlist"
```

**Client Implementation Notes:**
- Cascade deletion removes all playlist-track associations
- Original tracks remain in the library
- Operation cannot be undone

---

#### PUT /api/playlists/{playlistId}
**Description:** Update playlist metadata with support for cover image upload

**Authentication:** Not Required

**Request:**
- **Path Parameters:**
  - `playlistId` (integer, required): ID of the playlist to update
- **Content-Type:** `multipart/form-data`
- **Form Fields:**
  - `name` (string, required): New playlist name
  - `description` (string, optional): New playlist description
  - `cover` (file, optional): Cover image file upload

**Response:**

*Success (200 OK):*
```json
{
  "message": "Playlist updated successfully",
  "coverPath": "/path/to/uploaded/cover.jpg"
}
```

*Error (400 Bad Request):*
```
"Invalid playlist ID" or "Playlist name is required"
```

*Error (500 Internal Server Error):*
```
"Error updating playlist"
```

**Client Implementation Notes:**
- Use multipart form data for file uploads
- Cover images are stored in the server's static directory
- Maximum file size is 32MB
- Only name field is required; description and cover are optional

---

### Downloads

#### POST /api/download
**Description:** Start downloading audio from external URLs using yt-dlp

**Authentication:** Not Required

**Request:**
- **Headers:**
  - `Content-Type: application/json`
- **Request Body:**
```json
{
  "url": "https://www.youtube.com/watch?v=example",
  "title": "Custom Title",
  "artist": "Custom Artist"
}
```

**Response:**

*Success (200 OK):*
```json
{
  "job_id": "uuid-string",
  "status": "pending",
  "message": "Download started successfully"
}
```

*Error (400 Bad Request):*
```json
{
  "error": "URL is required"
}
```

*Error (503 Service Unavailable):*
```json
{
  "error": "Download functionality not available. Please install yt-dlp."
}
```

**Client Implementation Notes:**
- `url` field is required
- `title` and `artist` are optional; will be extracted from metadata if not provided
- Save the `job_id` to track download progress
- Downloads are processed asynchronously

---

#### GET /api/downloads
**Description:** Get status of all download jobs

**Authentication:** Not Required

**Request:**
- **Headers:** None required

**Response:**

*Success (200 OK):*
```json
[
  {
    "id": "uuid-string",
    "url": "https://www.youtube.com/watch?v=example",
    "title": "Video Title",
    "artist": "Artist Name",
    "status": "downloading",
    "progress": 75,
    "error": "",
    "output_path": "/path/to/file.mp3",
    "created_at": "2024-01-01T12:00:00Z",
    "completed_at": null
  }
]
```

*Error (503 Service Unavailable):*
```json
{
  "error": "Download functionality not available."
}
```

**Client Implementation Notes:**
- Status values: "pending", "downloading", "processing", "completed", "failed"
- Progress is a percentage (0-100) during download phase
- `completed_at` is null for incomplete jobs
- Poll this endpoint for real-time updates

---

#### GET /api/downloads/{jobId}
**Description:** Get status of a specific download job

**Authentication:** Not Required

**Request:**
- **Path Parameters:**
  - `jobId` (string, required): UUID of the download job

**Response:**

*Success (200 OK):*
```json
{
  "id": "uuid-string",
  "url": "https://www.youtube.com/watch?v=example",
  "title": "Video Title",
  "artist": "Artist Name",
  "status": "completed",
  "progress": 100,
  "error": "",
  "output_path": "/path/to/file.mp3",
  "created_at": "2024-01-01T12:00:00Z",
  "completed_at": "2024-01-01T12:05:00Z"
}
```

*Error (404 Not Found):*
```
"Download job not found"
```

**Client Implementation Notes:**
- Use this for tracking specific job progress
- More efficient than polling all jobs when tracking individual downloads

---

#### POST /api/validate-url
**Description:** Validate if a URL is supported for downloading without starting the download

**Authentication:** Not Required

**Request:**
- **Headers:**
  - `Content-Type: application/json`
- **Request Body:**
```json
{
  "url": "https://www.youtube.com/watch?v=example"
}
```

**Response:**

*Success (200 OK):*
```json
{
  "url": "https://www.youtube.com/watch?v=example",
  "valid": true,
  "message": "URL is valid and supported"
}
```

*Invalid URL:*
```json
{
  "url": "https://unsupported-site.com/video",
  "valid": false,
  "message": "URL not supported or invalid"
}
```

**Client Implementation Notes:**
- Use this to validate URLs before showing download options
- Faster than attempting a full download
- Supports 500+ sites via yt-dlp

---

### Static Content

#### GET /
**Description:** Serve the main web application interface

**Authentication:** Not Required

**Response:**
- **Content-Type:** `text/html`
- **Body:** Main HTML interface for the music server

#### GET /static/{file}
**Description:** Serve static assets (CSS, JavaScript, images)

**Authentication:** Not Required

**Request:**
- **Path Parameters:**
  - `file` (string): Path to static file

**Response:**
- **Content-Type:** Appropriate MIME type based on file extension
- **Body:** Static file content

---

## Data Models

### Track
```json
{
  "id": "integer - Unique track identifier",
  "title": "string - Track title",
  "artist": "string - Artist name",
  "album": "string - Album name", 
  "trackNumber": "integer - Track number in album",
  "duration": "integer - Duration in seconds",
  "fileSize": "integer - File size in bytes",
  "hasAlbumArt": "boolean - Whether album art is available",
  "albumArtId": "string - ID for album art retrieval"
}
```

### Playlist
```json
{
  "id": "integer - Unique playlist identifier",
  "name": "string - Playlist name",
  "description": "string - Optional description",
  "coverPath": "string - Path to cover image",
  "createdAt": "string - ISO 8601 timestamp",
  "trackCount": "integer - Number of tracks in playlist"
}
```

### Download Job
```json
{
  "id": "string - UUID job identifier",
  "url": "string - Source URL",
  "title": "string - Track title",
  "artist": "string - Artist name",
  "status": "string - Job status",
  "progress": "integer - Progress percentage (0-100)",
  "error": "string - Error message if failed",
  "output_path": "string - Path to downloaded file",
  "created_at": "string - ISO 8601 timestamp",
  "completed_at": "string or null - Completion timestamp"
}
```

## Error Codes

| Status Code | Description | Common Causes |
|-------------|-------------|---------------|
| 400 | Bad Request | Invalid JSON, missing required fields, invalid IDs |
| 404 | Not Found | Track not found, playlist not found, job not found |
| 405 | Method Not Allowed | Using wrong HTTP method for endpoint |
| 500 | Internal Server Error | Database errors, file system errors |
| 503 | Service Unavailable | yt-dlp not installed, downloader disabled |

## Rate Limiting
Currently, no rate limiting is implemented. However, download operations are limited by the `max_concurrent_downloads` configuration setting (default: 2).

## CORS Configuration
CORS support can be enabled/disabled via the `enable_cors` setting in the server configuration. When enabled, the server sends `Access-Control-Allow-Origin: *` headers.

## Examples

### Playing a Track
```javascript
// 1. Get tracks
const response = await fetch('/api/tracks');
const tracks = await response.json();

// 2. Set audio source and play
const audio = document.getElementById('audioPlayer');
audio.src = `/stream/${tracks[0].id}`;
audio.play();
```

### Creating and Managing Playlists
```javascript
// Create playlist
const createResponse = await fetch('/api/playlists/create', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    name: 'My Favorites',
    description: 'Best songs ever'
  })
});
const { id: playlistId } = await createResponse.json();

// Add track to playlist
await fetch(`/api/playlists/${playlistId}/tracks`, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ trackId: 1 })
});
```

### Downloading Music
```javascript
// Start download
const downloadResponse = await fetch('/api/download', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    url: 'https://www.youtube.com/watch?v=example'
  })
});
const { job_id } = await downloadResponse.json();

// Poll for status
const statusResponse = await fetch(`/api/downloads/${job_id}`);
const job = await statusResponse.json();
console.log(`Status: ${job.status}, Progress: ${job.progress}%`);
```
