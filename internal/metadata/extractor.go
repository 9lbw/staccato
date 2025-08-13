package metadata

import (
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"staccato/pkg/models"

	"github.com/dhowden/tag"
	"github.com/go-audio/wav"
	"github.com/mewkiz/flac"
	"github.com/sirupsen/logrus"
	"github.com/tcolgate/mp3"
)

// Extractor handles metadata extraction from audio files
type Extractor struct {
	supportedFormats []string
	logger           *logrus.Logger
	albumArtCache    map[string][]byte // Cache for album art
	albumArtMux      sync.RWMutex      // Mutex for album art cache
}

// NewExtractor creates a new metadata extractor
func NewExtractor(supportedFormats []string) *Extractor {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	return &Extractor{
		supportedFormats: supportedFormats,
		logger:           logger,
		albumArtCache:    make(map[string][]byte),
		albumArtMux:      sync.RWMutex{},
	}
}

// ExtractFromFile extracts metadata from an audio file
func (e *Extractor) ExtractFromFile(filePath string, id int) (models.Track, error) {
	startTime := time.Now()

	file, err := os.Open(filePath)
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"filePath": filePath,
			"error":    err.Error(),
		}).Error("Failed to open audio file")
		return models.Track{}, err
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"filePath": filePath,
			"error":    err.Error(),
		}).Error("Failed to get file stats")
		return models.Track{}, err
	}

	// Calculate duration
	duration, err := e.calculateDuration(filePath)
	if err != nil {
		e.logger.WithFields(logrus.Fields{
			"filePath": filePath,
			"error":    err.Error(),
		}).Warn("Failed to calculate duration, setting to 0")
		duration = 0
	}

	// Extract metadata using the tag library
	metadata, err := tag.ReadFrom(file)
	if err != nil {
		// If metadata extraction fails, use filename
		filename := filepath.Base(filePath)
		name := strings.TrimSuffix(filename, filepath.Ext(filename))

		e.logger.WithFields(logrus.Fields{
			"filePath": filePath,
			"error":    err.Error(),
		}).Warn("Failed to extract metadata, using filename")

		return models.Track{
			ID:          id,
			Title:       name,
			Artist:      "Unknown Artist",
			Album:       "Unknown Album",
			TrackNumber: 0,
			Duration:    duration,
			FilePath:    filePath,
			FileSize:    stat.Size(),
		}, nil
	}

	title := metadata.Title()
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	}

	artist := metadata.Artist()
	if artist == "" {
		artist = "Unknown Artist"
	}

	album := metadata.Album()
	if album == "" {
		album = "Unknown Album"
	}

	// Extract track number
	trackNum, _ := metadata.Track()

	// Extract album art
	albumArtID, hasAlbumArt := e.extractAlbumArt(metadata)

	processingTime := time.Since(startTime)
	e.logger.WithFields(logrus.Fields{
		"filePath":       filePath,
		"title":          title,
		"artist":         artist,
		"album":          album,
		"duration":       duration,
		"hasAlbumArt":    hasAlbumArt,
		"processingTime": processingTime,
	}).Debug("Successfully extracted metadata")

	return models.Track{
		ID:          id,
		Title:       title,
		Artist:      artist,
		Album:       album,
		TrackNumber: trackNum,
		Duration:    duration,
		FilePath:    filePath,
		FileSize:    stat.Size(),
		HasAlbumArt: hasAlbumArt,
		AlbumArtID:  albumArtID,
	}, nil
}

// calculateDuration calculates the duration of an audio file in seconds
func (e *Extractor) calculateDuration(filePath string) (int, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp3":
		return e.durationMP3(filePath)
	case ".flac":
		return e.durationFLAC(filePath)
	case ".wav":
		return e.durationWAV(filePath)
	case ".m4a":
		return e.durationM4A(filePath)
	default:
		return 0, fmt.Errorf("unsupported format: %s", ext)
	}
}

// MP3 duration using frame decoding; fallback to average bitrate estimation only if frames fail entirely.
func (e *Extractor) durationMP3(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	dec := mp3.NewDecoder(f)
	var total time.Duration
	var skipped int
	frames := 0
	for {
		var fr mp3.Frame
		if err := dec.Decode(&fr, &skipped); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if frames == 0 { // could not decode any frame
				return e.estimateFromFileSize(path, 192000) // assume 192 kbps = 192000 bps
			}
			break // partial decode; use what we have
		}
		total += fr.Duration()
		frames++
	}
	return int(total.Seconds()), nil
}

// FLAC duration via STREAMINFO metadata block
func (e *Extractor) durationFLAC(path string) (int, error) {
	stream, err := flac.ParseFile(path)
	if err != nil {
		return 0, err
	}
	si := stream.Info
	if si.NSamples > 0 && si.SampleRate > 0 {
		secs := float64(si.NSamples) / float64(si.SampleRate)
		return int(secs + 0.5), nil
	}
	return 0, fmt.Errorf("flac stream missing sample info")
}

// WAV duration using go-audio/wav to read header
func (e *Extractor) durationWAV(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	dec := wav.NewDecoder(f)
	if !dec.IsValidFile() {
		return 0, fmt.Errorf("invalid wav file")
	}
	if dec.SampleRate == 0 || dec.BitDepth == 0 || dec.NumChans == 0 {
		return 0, fmt.Errorf("invalid wav header")
	}
	// Approximate using file size; full sample count may require decoding all samples.
	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	headerSize := int64(44)
	pcmBytes := st.Size() - headerSize
	if pcmBytes < 0 {
		pcmBytes = 0
	}
	bytesPerSampleFrame := int64(dec.BitDepth/8) * int64(dec.NumChans)
	if bytesPerSampleFrame <= 0 {
		return 0, fmt.Errorf("invalid sample frame size")
	}
	sampleFrames := pcmBytes / bytesPerSampleFrame
	secs := float64(sampleFrames) / float64(dec.SampleRate)
	return int(secs + 0.5), nil
}

// M4A (AAC in MP4) minimal duration parsing: read 'mvhd' timescale & duration.
// Lightweight manual atom scan to avoid pulling large dep. Best-effort.
func (e *Extractor) durationM4A(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	for {
		head := make([]byte, 8)
		if _, err := io.ReadFull(f, head); err != nil {
			return 0, err
		}
		size := binary.BigEndian.Uint32(head[0:4])
		atom := string(head[4:8])
		if size < 8 {
			return 0, fmt.Errorf("invalid atom size")
		}
		if atom == "moov" {
			// scan inside moov for mvhd
			limit := int64(size) - 8
			// position inside moov (not used)
			for read := int64(0); read < limit; {
				subHead := make([]byte, 8)
				if _, err := io.ReadFull(f, subHead); err != nil {
					return 0, err
				}
				subSize := binary.BigEndian.Uint32(subHead[0:4])
				subAtom := string(subHead[4:8])
				if subAtom == "mvhd" {
					version := make([]byte, 1)
					if _, err := io.ReadFull(f, version); err != nil {
						return 0, err
					}
					var skip int64
					if version[0] == 1 { // 64-bit
						skip = 3 + 8 + 8 // flags + creation + mod times (64-bit)
					} else {
						skip = 3 + 4 + 4 // flags + times (32-bit)
					}
					if _, err := f.Seek(skip, io.SeekCurrent); err != nil {
						return 0, err
					}
					tsBuf := make([]byte, 4)
					if _, err := io.ReadFull(f, tsBuf); err != nil {
						return 0, err
					}
					timescale := binary.BigEndian.Uint32(tsBuf)
					durBuf := make([]byte, 4)
					if _, err := io.ReadFull(f, durBuf); err != nil {
						return 0, err
					}
					durUnits := binary.BigEndian.Uint32(durBuf)
					if timescale == 0 {
						return 0, fmt.Errorf("invalid timescale")
					}
					secs := float64(durUnits) / float64(timescale)
					return int(secs + 0.5), nil
				}
				// skip remainder of sub atom
				if subSize < 8 {
					return 0, fmt.Errorf("invalid sub-atom size")
				}
				if _, err := f.Seek(int64(subSize)-8, io.SeekCurrent); err != nil {
					return 0, err
				}
				read += int64(subSize)
			}
			break
		}
		// skip rest of atom
		if _, err := f.Seek(int64(size)-8, io.SeekCurrent); err != nil {
			return 0, err
		}
	}
	return 0, fmt.Errorf("mvhd atom not found")
}

// estimateFromFileSize provides last-resort estimation if parsing fails.
func (e *Extractor) estimateFromFileSize(path string, bitrate int) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if bitrate <= 0 {
		return 0, fmt.Errorf("invalid bitrate")
	}
	dur := (st.Size() * 8) / int64(bitrate)
	return int(dur), nil
}

// extractAlbumArt extracts album art from metadata or looks for cover files
func (e *Extractor) extractAlbumArt(metadata tag.Metadata) (string, bool) {
	// First try to extract embedded album art
	if metadata != nil {
		if picture := metadata.Picture(); picture != nil {
			// Create a unique ID for this album art based on content hash
			hash := md5.Sum(picture.Data)
			artID := fmt.Sprintf("%x", hash)

			// Cache the album art data safely
			e.albumArtMux.Lock()
			e.albumArtCache[artID] = picture.Data
			e.albumArtMux.Unlock()

			return artID, true
		}
	}

	// No embedded album art found; skip directory cover fallback to avoid incorrect cover assignment
	return "", false
}

// GetAlbumArt retrieves cached album art by ID
func (e *Extractor) GetAlbumArt(artID string) ([]byte, bool) {
	// Thread-safe read
	e.albumArtMux.RLock()
	data, exists := e.albumArtCache[artID]
	e.albumArtMux.RUnlock()
	return data, exists
}

// GetAlbumArtMimeType guesses MIME type from album art data
func (e *Extractor) GetAlbumArtMimeType(data []byte) string {
	if len(data) < 4 {
		return "application/octet-stream"
	}

	// Check for common image formats
	if data[0] == 0xFF && data[1] == 0xD8 {
		return "image/jpeg"
	}
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return "image/gif"
	}

	return "application/octet-stream"
}

// IsAudioFile checks if a file is a supported audio format
func (e *Extractor) IsAudioFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, format := range e.supportedFormats {
		if ext == format {
			return true
		}
	}
	return false
}

// GetContentType returns the MIME type for an audio file
func (e *Extractor) GetContentType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp3":
		return "audio/mpeg"
	case ".flac":
		return "audio/flac"
	case ".wav":
		return "audio/wav"
	case ".m4a":
		return "audio/mp4"
	default:
		return "application/octet-stream"
	}
}
