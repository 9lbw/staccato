package server

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture status code & size.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(data)
	rw.size += size
	return size, err
}

// requestLoggingMiddleware logs HTTP requests (if enabled) with latency & size.
func (ms *MusicServer) requestLoggingMiddleware(next http.Handler) http.Handler {
	if !ms.config.Logging.RequestLogging {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer that captures status code and size
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     200, // Default status code
		}

		// Call the next handler
		next.ServeHTTP(rw, r)

		// Log the request
		duration := time.Since(start)

		// Skip logging for static assets and health checks to reduce noise
		if ms.shouldLogRequest(r.URL.Path) {
			log.Printf("[%s] %s %s - %d %s (%v)",
				r.Method,
				r.URL.Path,
				r.RemoteAddr,
				rw.statusCode,
				formatBytes(rw.size),
				duration.Round(time.Millisecond),
			)
		}
	})
}

// corsMiddleware injects CORS headers if enabled in configuration. Keeps existing
// behavior (only Access-Control-Allow-Origin: *). Does not introduce preflight handling
// to avoid functional changes.
func (ms *MusicServer) corsMiddleware(next http.Handler) http.Handler {
	if !ms.config.Server.EnableCORS {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		next.ServeHTTP(w, r)
	})
}

// shouldLogRequest filters noisy paths from request logging output.
func (ms *MusicServer) shouldLogRequest(path string) bool {
	// Skip logging for common static assets and frequent endpoints
	skipPaths := []string{
		"/static/css/",
		"/static/js/",
		"/static/images/",
		"/favicon.ico",
	}

	for _, skipPath := range skipPaths {
		if len(path) >= len(skipPath) && path[:len(skipPath)] == skipPath {
			return false
		}
	}

	return true
}

// formatBytes provides a simple approximate human-readable size.
func formatBytes(bytes int) string {
	if bytes == 0 {
		return "0B"
	}

	const unit = 1024
	if bytes < unit {
		return "< 1KB"
	}

	div, exp := int64(unit), 0
	for n := int64(bytes) / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}

	result := int64(bytes) / div
	return fmt.Sprintf("%d%s", result, units[exp])
}

// panicRecoveryMiddleware intercepts panics returning HTTP 500 without crashing the process.
func (ms *MusicServer) panicRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("PANIC in %s %s: %v", r.Method, r.URL.Path, err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
