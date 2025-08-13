package downloader

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"staccato/internal/config"
	"staccato/internal/database"
	"staccato/internal/metadata"

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

// DownloadJob represents a download job's current (possibly transient) state.
// Fields are JSON tagged for direct API serialization.
type DownloadJob struct {
	ID          string         `json:"id"`
	URL         string         `json:"url"`
	Title       string         `json:"title"`
	Artist      string         `json:"artist"`
	Status      DownloadStatus `json:"status"`
	Progress    int            `json:"progress"`
	Error       string         `json:"error,omitempty"`
	OutputPath  string         `json:"output_path,omitempty"`
	Speed       string         `json:"speed,omitempty"`
	ETASeconds  int            `json:"eta_seconds,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// Downloader orchestrates concurrent yt-dlp based downloads, tracking progress
// and (optionally) ingesting completed files into the library database.
type Downloader struct {
	config    *config.Config
	jobs      map[string]*DownloadJob
	jobsMux   sync.RWMutex
	ytDlpPath string
	sem       chan struct{} // concurrency semaphore
	db        *database.Database
	extractor *metadata.Extractor
}

// NewDownloader constructs a Downloader, validating presence of yt-dlp and
// preparing internal state. Returns an error if yt-dlp cannot be located.
func NewDownloader(cfg *config.Config) (*Downloader, error) {
	d := &Downloader{
		config: cfg,
		jobs:   make(map[string]*DownloadJob),
	}
	// Set up semaphore for concurrency limit
	max := cfg.Downloader.MaxConcurrent
	if max < 1 {
		max = 1
	}
	d.sem = make(chan struct{}, max)

	// Check if yt-dlp is available
	if err := d.checkYtDlp(); err != nil {
		return nil, fmt.Errorf("yt-dlp not available: %w", err)
	}

	return d, nil
}

// AttachIngest wires automatic metadata extraction & DB insertion for
// successfully downloaded files. Can be called after construction.
func (d *Downloader) AttachIngest(db *database.Database, extractor *metadata.Extractor) {
	d.db = db
	d.extractor = extractor
}

// checkYtDlp attempts to discover an executable yt-dlp binary from a small
// set of common names/locations. The first successful path is cached.
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

// DownloadFromURL schedules a new download job. It returns immediately with
// the created job whose status will transition asynchronously.
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

	// Acquire semaphore slot then start download in background
	go func() {
		// Concurrency control
		d.sem <- struct{}{}
		defer func() { <-d.sem }()
		d.processDownload(job)
	}()

	return job, nil
}

// processDownload executes the yt-dlp command pipeline for a single job and
// updates status progressively. Must run in its own goroutine.
func (d *Downloader) processDownload(job *DownloadJob) {
	d.updateJobStatus(job.ID, StatusDownloading, 0, "")

	// Retrieve remote metadata
	meta, err := d.getMetadata(job.URL)
	if err != nil {
		d.updateJobStatus(job.ID, StatusFailed, 0, fmt.Sprintf("Failed metadata: %v", err))
		return
	}
	if job.Title == "" {
		job.Title = meta.Title
	}
	if job.Artist == "" {
		if meta.Artist != "" {
			job.Artist = meta.Artist
		} else {
			job.Artist = meta.Uploader
		}
	}

	// Prepare output filename
	safeTitle := d.sanitizeFilename(job.Title)
	safeArtist := d.sanitizeFilename(job.Artist)
	filename := fmt.Sprintf("%s - %s.%%(ext)s", safeArtist, safeTitle)
	outputPath := filepath.Join(d.config.Music.LibraryPath, filename)

	// Build command with progress-friendly output
	cmd := exec.Command(d.ytDlpPath,
		"--extract-audio",
		"--audio-format", d.config.Downloader.AudioFormat,
		"--audio-quality", d.config.Downloader.AudioQuality,
		"--no-playlist",
		"--newline", // ensures progress lines
		"--output", outputPath,
		job.URL,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		d.updateJobStatus(job.ID, StatusFailed, 0, fmt.Sprintf("pipe error: %v", err))
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		d.updateJobStatus(job.ID, StatusFailed, 0, fmt.Sprintf("pipe error: %v", err))
		return
	}

	if err := cmd.Start(); err != nil {
		d.updateJobStatus(job.ID, StatusFailed, 0, fmt.Sprintf("start error: %v", err))
		return
	}

	// Example yt-dlp line: [download]  45.3% of 3.33MiB at 512.34KiB/s ETA 00:12
	progressRe := regexp.MustCompile(`(?i)\[download\]\s+([0-9\.]+)%.*?at\s+([0-9\.]+[KMG]i?B/s).*?ETA\s+([0-9:]{2,8})`)
	simplePercentRe := regexp.MustCompile(`(?i)\[download\]\s+([0-9\.]+)%`)
	parseETA := func(etaStr string) int { // HH:MM:SS or MM:SS
		parts := strings.Split(etaStr, ":")
		if len(parts) == 2 { // mm:ss
			var m, s int
			fmt.Sscanf(parts[0], "%d", &m)
			fmt.Sscanf(parts[1], "%d", &s)
			return m*60 + s
		} else if len(parts) == 3 { // hh:mm:ss
			var h, m, s int
			fmt.Sscanf(parts[0], "%d", &h)
			fmt.Sscanf(parts[1], "%d", &m)
			fmt.Sscanf(parts[2], "%d", &s)
			return h*3600 + m*60 + s
		}
		return -1
	}
	updateProgress := func(line string) {
		if m := progressRe.FindStringSubmatch(line); len(m) == 4 {
			pStr := strings.TrimSpace(m[1])
			var val float64
			fmt.Sscanf(pStr, "%f", &val)
			speed := strings.TrimSpace(m[2])
			etaSec := parseETA(strings.TrimSpace(m[3]))
			if val >= 0 && val <= 100 {
				status := StatusDownloading
				if val > 97 {
					status = StatusProcessing
				}
				d.updateJobStatus(job.ID, status, int(val), "")
				d.updateJobProgress(job.ID, int(val), speed, etaSec)
			}
			return
		}
		if m := simplePercentRe.FindStringSubmatch(line); len(m) == 2 { // fallback
			pStr := strings.TrimSpace(m[1])
			var val float64
			fmt.Sscanf(pStr, "%f", &val)
			if val >= 0 && val <= 100 {
				status := StatusDownloading
				if val > 97 {
					status = StatusProcessing
				}
				d.updateJobStatus(job.ID, status, int(val), "")
			}
		}
	}

	// Scan stdout & stderr concurrently
	var wg sync.WaitGroup
	scan := func(r io.Reader) {
		defer wg.Done()
		s := bufio.NewScanner(r)
		for s.Scan() {
			line := s.Text()
			updateProgress(line)
		}
	}
	wg.Add(2)
	go scan(stdout)
	go scan(stderr)
	wg.Wait()

	// Wait for command completion
	if err := cmd.Wait(); err != nil {
		// Capture possible error details
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			d.updateJobStatus(job.ID, StatusFailed, job.Progress, fmt.Sprintf("yt-dlp error: %s", strings.TrimSpace(string(exitErr.Stderr))))
		} else {
			d.updateJobStatus(job.ID, StatusFailed, job.Progress, fmt.Sprintf("yt-dlp failed: %v", err))
		}
		return
	}

	// Determine actual file path
	actualPath := strings.Replace(outputPath, ".%(ext)s", "."+d.config.Downloader.AudioFormat, 1)
	job.OutputPath = actualPath

	// Mark complete
	d.updateJobStatus(job.ID, StatusCompleted, 100, "")
	now := time.Now()
	job.CompletedAt = &now

	// Ingest into library
	if d.db != nil && d.extractor != nil {
		track, err := d.extractor.ExtractFromFile(actualPath, 0)
		if err == nil {
			_, _ = d.db.InsertTrack(track)
		}
	}
}

// VideoMetadata represents metadata extracted from a video
type VideoMetadata struct {
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	Uploader string  `json:"uploader"`
	Duration float64 `json:"duration"`
}

// getMetadata performs a metadata-only probe via yt-dlp (--dump-json) to
// pre-populate job fields before the actual download.
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

// sanitizeFilename removes characters disallowed or awkward for typical
// filesystems, producing a safe base name.
func (d *Downloader) sanitizeFilename(filename string) string {
	// Replace invalid characters with underscores
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := filename
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "_")
	}
	return strings.TrimSpace(result)
}

// updateJobStatus centralizes status mutation and persistence (if DB attached).
// It is concurrency-safe.
func (d *Downloader) updateJobStatus(jobID string, status DownloadStatus, progress int, errorMsg string) {
	d.jobsMux.Lock()
	defer d.jobsMux.Unlock()

	if job, exists := d.jobs[jobID]; exists {
		job.Status = status
		job.Progress = progress
		if errorMsg != "" {
			job.Error = errorMsg
		}
		// Persist if DB available
		if d.db != nil {
			_ = d.db.UpsertDownloadJob(job.ID, job.URL, job.Title, job.Artist, string(job.Status), job.Progress, job.Error, job.OutputPath, job.Speed, job.ETASeconds, &job.CreatedAt, job.CompletedAt)
		}
	}
}

// updateJobProgress updates frequently changing progress fields without
// altering textual error. Keeps DB row in sync.
func (d *Downloader) updateJobProgress(jobID string, progress int, speed string, etaSeconds int) {
	d.jobsMux.Lock()
	defer d.jobsMux.Unlock()
	if job, exists := d.jobs[jobID]; exists {
		job.Progress = progress
		if speed != "" {
			job.Speed = speed
		}
		if etaSeconds >= 0 {
			job.ETASeconds = etaSeconds
		}
		if d.db != nil {
			_ = d.db.UpsertDownloadJob(job.ID, job.URL, job.Title, job.Artist, string(job.Status), job.Progress, job.Error, job.OutputPath, job.Speed, job.ETASeconds, &job.CreatedAt, job.CompletedAt)
		}
	}
}

// GetJob returns a snapshot pointer of the job (callers must treat it as
// read-only unless additional locking is done).
func (d *Downloader) GetJob(jobID string) (*DownloadJob, bool) {
	d.jobsMux.RLock()
	defer d.jobsMux.RUnlock()

	job, exists := d.jobs[jobID]
	return job, exists
}

// GetAllJobs returns shallow copies of in-memory job pointers for inspection.
func (d *Downloader) GetAllJobs() []*DownloadJob {
	d.jobsMux.RLock()
	defer d.jobsMux.RUnlock()

	jobs := make([]*DownloadJob, 0, len(d.jobs))
	for _, job := range d.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// CleanupCompletedJobs removes terminal state jobs older than maxAge to bound
// memory usage of the in-memory job map.
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

// ValidateURL performs a light yt-dlp simulation to confirm basic support for
// the provided URL.
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
