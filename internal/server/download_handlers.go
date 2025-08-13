package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// handleDownloadMusic handles music download requests
func (ms *MusicServer) handleDownloadMusic(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)

	// Check if downloader is available
	if ms.downloader == nil {
		response := map[string]interface{}{
			"error": "Download functionality not available. Please install yt-dlp.",
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response)
		return
	}

	var req struct {
		URL    string `json:"url"`
		Title  string `json:"title,omitempty"`
		Artist string `json:"artist,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Validate URL
	if err := ms.downloader.ValidateURL(req.URL); err != nil {
		response := map[string]interface{}{
			"error": fmt.Sprintf("Invalid URL: %v", err),
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Start download
	job, err := ms.downloader.DownloadFromURL(req.URL, req.Title, req.Artist)
	if err != nil {
		response := map[string]interface{}{
			"error": fmt.Sprintf("Failed to start download: %v", err),
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := map[string]interface{}{
		"job_id":  job.ID,
		"status":  job.Status,
		"message": "Download started successfully",
	}
	json.NewEncoder(w).Encode(response)
}

// handleGetDownloads handles requests to get download status
func (ms *MusicServer) handleGetDownloads(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)

	// Check if downloader is available
	if ms.downloader == nil {
		response := map[string]interface{}{
			"error": "Download functionality not available.",
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get specific job ID from URL path if provided
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) >= 4 && pathParts[3] != "" {
		// Get specific job
		jobID := pathParts[3]
		job, exists := ms.downloader.GetJob(jobID)
		if !exists {
			http.Error(w, "Download job not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(job)
	} else {
		// Get all in-memory jobs
		jobs := ms.downloader.GetAllJobs()
		json.NewEncoder(w).Encode(jobs)
	}
}

// handleCleanupDownloads removes completed/failed jobs older than ?age= (minutes, default 60)
func (ms *MusicServer) handleCleanupDownloads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)
	if ms.downloader == nil {
		http.Error(w, "Downloader not available", http.StatusServiceUnavailable)
		return
	}
	ageStr := r.URL.Query().Get("age")
	ageMinutes := 60
	if ageStr != "" {
		fmt.Sscanf(ageStr, "%d", &ageMinutes)
	}
	if ageMinutes < 1 {
		ageMinutes = 1
	}
	ms.downloader.CleanupCompletedJobs(time.Duration(ageMinutes) * time.Minute)
	json.NewEncoder(w).Encode(map[string]any{"message": "cleanup complete", "age_minutes": ageMinutes})
}

// handleValidateURL handles URL validation requests
func (ms *MusicServer) handleValidateURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ms.setCORSHeaders(w)

	// Check if downloader is available
	if ms.downloader == nil {
		response := map[string]interface{}{
			"error": "Download functionality not available.",
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response)
		return
	}

	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// Validate URL
	err := ms.downloader.ValidateURL(req.URL)
	response := map[string]interface{}{
		"url":     req.URL,
		"valid":   err == nil,
		"message": "",
	}

	if err != nil {
		response["message"] = err.Error()
	} else {
		response["message"] = "URL is valid and supported"
	}

	json.NewEncoder(w).Encode(response)
}
