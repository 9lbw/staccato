package server

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

const (
	// Buffer size for streaming (64KB)
	streamBufferSize = 64 * 1024
)

// OptimizedStreamHandler provides high-performance audio streaming with buffering and caching
func (ms *MusicServer) OptimizedStreamHandler(w http.ResponseWriter, r *http.Request, filePath string, contentType string) error {
	// Get file info
	stat, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("error reading file info: %v", err)
	}

	fileSize := stat.Size()
	modTime := stat.ModTime().Unix()

	// Open file with read buffer
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	// Set caching headers
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("ETag", fmt.Sprintf(`"%d-%d"`, modTime, fileSize))

	// Check if client has cached version
	if ms.checkNotModified(w, r, modTime) {
		return nil
	}

	// Set streaming headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	ms.setCORSHeaders(w)

	// Handle range requests
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		ms.handleRangeRequest(w, r, file, fileSize, rangeHeader)
		return nil
	}

	// Stream entire file with optimized buffering
	w.Header().Set("Content-Length", strconv.FormatInt(fileSize, 10))

	bufferedReader := bufio.NewReaderSize(file, streamBufferSize)
	buffer := make([]byte, streamBufferSize)

	_, err = io.CopyBuffer(w, bufferedReader, buffer)
	if err != nil {
		return fmt.Errorf("error streaming file: %v", err)
	}

	return nil
}

// checkNotModified checks if the client has a cached version
func (ms *MusicServer) checkNotModified(w http.ResponseWriter, r *http.Request, modTime int64) bool {
	// Check ETag
	etag := fmt.Sprintf(`"%d"`, modTime)
	if match := r.Header.Get("If-None-Match"); match == etag {
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	return false
}
