package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

// authMiddleware checks if the user is authenticated for protected routes
func (ms *MusicServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth check if authentication is disabled
		if !ms.authService.IsEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		// Allow access to auth-related endpoints and static assets
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// Check for valid session
		sessionManager := ms.authService.GetSessionManager()
		session, valid := sessionManager.GetSessionFromRequest(r)
		if !valid {
			// Redirect to login page for browser requests
			if isBrowserRequest(r) {
				http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
				return
			}
			// Return 401 for API requests
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"})
			return
		}

		// Refresh session on each request
		ms.authService.RefreshSession(session.ID)

		// Add user info to request context if needed
		// ctx := context.WithValue(r.Context(), "user", session.Username)
		// r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// isPublicPath checks if a path should be accessible without authentication
func isPublicPath(path string) bool {
	publicPaths := []string{
		"/login",
		"/api/auth/login",
		"/api/auth/logout",
		"/static/",
		"/health",
	}

	for _, publicPath := range publicPaths {
		if strings.HasPrefix(path, publicPath) {
			return true
		}
	}

	return false
}

// isBrowserRequest checks if the request is from a browser (vs API client)
func isBrowserRequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html")
}

// handleLogin serves the login page
func (ms *MusicServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// Check if user is already logged in
		if ms.authService.IsEnabled() {
			sessionManager := ms.authService.GetSessionManager()
			if _, valid := sessionManager.GetSessionFromRequest(r); valid {
				http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
				return
			}
		}

		// Serve login page
		http.ServeFile(w, r, ms.config.Server.StaticDir+"/login.html")
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleAuthLogin handles login API requests
func (ms *MusicServer) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if credentials.Username == "" || credentials.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Username and password required"})
		return
	}

	session, err := ms.authService.Login(credentials.Username, credentials.Password)
	if err != nil {
		ms.logger.WithError(err).WithField("username", credentials.Username).Warn("Failed login attempt")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid credentials"})
		return
	}

	// Set session cookie
	sessionManager := ms.authService.GetSessionManager()
	sessionManager.SetSessionCookie(w, session)

	ms.logger.WithField("username", credentials.Username).Info("User logged in successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// handleAuthLogout handles logout requests
func (ms *MusicServer) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session from request
	sessionManager := ms.authService.GetSessionManager()
	session, valid := sessionManager.GetSessionFromRequest(r)
	if valid {
		// Invalidate session
		ms.authService.Logout(session.ID)
		ms.logger.WithField("username", session.Username).Info("User logged out")
	}

	// Clear session cookie
	sessionManager.ClearSessionCookie(w)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
