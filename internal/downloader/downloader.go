package downloader

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"spotigo/internal/config"

	"github.com/google/uuid"
)

// DownloadStatus represents the status of a download
type DownloadStatus string

const (
	StatusPending     DownloadStatus = "pending"
	StatusDownloading DownloadStatus = "downloading"
	StatusProcessing  DownloadStatus = "processing"
	StatusCompleted   DownloadStatus = "completed"
	StatusFailed      DownloadStatus = "failed"
)

// DownloadJob represents a download job
type DownloadJob struct {
	ID          string         `json:"id"`
	URL         string         `json:"url"`
	Title       string         `json:"title"`
	Artist      string         `json:"artist"`
	Status      DownloadStatus `json:"status"`
	Progress    int            `json:"progress"`
	Error       string         `json:"error,omitempty"`
	OutputPath  string         `json:"output_path,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// Downloader handles music downloads from various sources
type Downloader struct {
	config    *config.Config
	jobs      map[string]*DownloadJob
	jobsMux   sync.RWMutex
	ytDlpPath string
}

// NewDownloader creates a new downloader instance
func NewDownloader(cfg *config.Config) (*Downloader, error) {
	d := &Downloader{
		config: cfg,
		jobs:   make(map[string]*DownloadJob),
	}

	// Check if yt-dlp is available
	if err := d.checkYtDlp(); err != nil {
		return nil, fmt.Errorf("yt-dlp not available: %w", err)
	}

	return d, nil
}

// checkYtDlp verifies that yt-dlp is installed and accessible
func (d *Downloader) checkYtDlp() error {
	// Try different possible locations for yt-dlp
	possiblePaths := []string{"yt-dlp", "yt-dlp.exe", "./yt-dlp", "./yt-dlp.exe"}

	for _, path := range possiblePaths {
		if _, err := exec.LookPath(path); err == nil {
			d.ytDlpPath = path
			return nil
		}
	}

	return fmt.Errorf("yt-dlp not found in PATH. Please install yt-dlp")
}

// DownloadFromURL starts a download from a given URL
func (d *Downloader) DownloadFromURL(url, customTitle, customArtist string) (*DownloadJob, error) {
	job := &DownloadJob{
		ID:        uuid.New().String(),
		URL:       url,
		Title:     customTitle,
		Artist:    customArtist,
		Status:    StatusPending,
		Progress:  0,
		CreatedAt: time.Now(),
	}

	d.jobsMux.Lock()
	d.jobs[job.ID] = job
	d.jobsMux.Unlock()

	// Start download in background
	go d.processDownload(job)

	return job, nil
}

// processDownload handles the actual download process
func (d *Downloader) processDownload(job *DownloadJob) {
	d.updateJobStatus(job.ID, StatusDownloading, 0, "")

	// First, get metadata about the video
	metadata, err := d.getMetadata(job.URL)
	if err != nil {
		d.updateJobStatus(job.ID, StatusFailed, 0, fmt.Sprintf("Failed to get metadata: %v", err))
		return
	}

	// Use custom title/artist if provided, otherwise use metadata
	if job.Title == "" {
		job.Title = metadata.Title
	}
	if job.Artist == "" {
		job.Artist = metadata.Artist
		if job.Artist == "" {
			job.Artist = metadata.Uploader
		}
	}

	// Create safe filename
	safeTitle := d.sanitizeFilename(job.Title)
	safeArtist := d.sanitizeFilename(job.Artist)
	filename := fmt.Sprintf("%s - %s.%%(ext)s", safeArtist, safeTitle)
	outputPath := filepath.Join(d.config.Music.LibraryPath, filename)

	d.updateJobStatus(job.ID, StatusProcessing, 25, "Downloading audio...")

	// Download the audio
	cmd := exec.Command(d.ytDlpPath,
		"--extract-audio",
		"--audio-format", d.config.Downloader.AudioFormat,
		"--audio-quality", d.config.Downloader.AudioQuality,
		"--output", outputPath,
		"--no-playlist",
		job.URL,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		d.updateJobStatus(job.ID, StatusFailed, 0, fmt.Sprintf("Download failed: %v\nOutput: %s", err, string(output)))
		return
	}

	// Find the actual output file (yt-dlp replaces %(ext)s with actual extension)
	actualPath := strings.Replace(outputPath, ".%(ext)s", "."+d.config.Downloader.AudioFormat, 1)
	job.OutputPath = actualPath

	d.updateJobStatus(job.ID, StatusCompleted, 100, "")
	now := time.Now()
	job.CompletedAt = &now
}

// VideoMetadata represents metadata extracted from a video
type VideoMetadata struct {
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Uploader string `json:"uploader"`
	Duration int    `json:"duration"`
}

// getMetadata extracts metadata from a URL without downloading
func (d *Downloader) getMetadata(url string) (*VideoMetadata, error) {
	cmd := exec.Command(d.ytDlpPath,
		"--dump-json",
		"--no-playlist",
		url,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	var metadata VideoMetadata
	if err := json.Unmarshal(output, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &metadata, nil
}

// sanitizeFilename removes invalid characters from filenames
func (d *Downloader) sanitizeFilename(filename string) string {
	// Replace invalid characters with underscores
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := filename
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}
	return strings.TrimSpace(result)
}

// updateJobStatus updates the status of a download job
func (d *Downloader) updateJobStatus(jobID string, status DownloadStatus, progress int, errorMsg string) {
	d.jobsMux.Lock()
	defer d.jobsMux.Unlock()

	if job, exists := d.jobs[jobID]; exists {
		job.Status = status
		job.Progress = progress
		if errorMsg != "" {
			job.Error = errorMsg
		}
	}
}

// GetJob returns a download job by ID
func (d *Downloader) GetJob(jobID string) (*DownloadJob, bool) {
	d.jobsMux.RLock()
	defer d.jobsMux.RUnlock()

	job, exists := d.jobs[jobID]
	return job, exists
}

// GetAllJobs returns all download jobs
func (d *Downloader) GetAllJobs() []*DownloadJob {
	d.jobsMux.RLock()
	defer d.jobsMux.RUnlock()

	jobs := make([]*DownloadJob, 0, len(d.jobs))
	for _, job := range d.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// CleanupCompletedJobs removes completed jobs older than specified duration
func (d *Downloader) CleanupCompletedJobs(maxAge time.Duration) {
	d.jobsMux.Lock()
	defer d.jobsMux.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, job := range d.jobs {
		if job.Status == StatusCompleted || job.Status == StatusFailed {
			if job.CompletedAt != nil && job.CompletedAt.Before(cutoff) {
				delete(d.jobs, id)
			}
		}
	}
}

// ValidateURL checks if a URL is supported for downloading
func (d *Downloader) ValidateURL(url string) error {
	// Basic URL validation
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("invalid URL: must start with http:// or https://")
	}

	// Check if yt-dlp can handle this URL
	cmd := exec.Command(d.ytDlpPath, "--simulate", "--no-playlist", url)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("URL not supported or invalid: %w", err)
	}

	return nil
}
