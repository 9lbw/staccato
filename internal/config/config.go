package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config represents the application configuration
type Config struct {
	Server     ServerConfig     `toml:"server"`
	Database   DatabaseConfig   `toml:"database"`
	Music      MusicConfig      `toml:"music"`
	Logging    LoggingConfig    `toml:"logging"`
	Downloader DownloaderConfig `toml:"downloader"`
	Ngrok      NgrokConfig      `toml:"ngrok"`
	Discord    DiscordConfig    `toml:"discord"`
}

// ServerConfig contains server-related configuration
type ServerConfig struct {
	Port        string `toml:"port"`
	Host        string `toml:"host"`
	StaticDir   string `toml:"static_dir"`
	EnableCORS  bool   `toml:"enable_cors"`
	ReadTimeout int    `toml:"read_timeout_seconds"`
}

// DatabaseConfig contains database-related configuration
type DatabaseConfig struct {
	Path           string `toml:"path"`
	MaxConnections int    `toml:"max_connections"`
}

// MusicConfig contains music library configuration
type MusicConfig struct {
	LibraryPath      string   `toml:"library_path"`
	SupportedFormats []string `toml:"supported_formats"`
	WatchForChanges  bool     `toml:"watch_for_changes"`
	ScanOnStartup    bool     `toml:"scan_on_startup"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
	File   string `toml:"file"`
}

// DownloaderConfig contains music download configuration
type DownloaderConfig struct {
	Enabled        bool     `toml:"enabled"`
	YtDlpPath      string   `toml:"yt_dlp_path"`
	MaxConcurrent  int      `toml:"max_concurrent_downloads"`
	AudioFormat    string   `toml:"audio_format"`
	AudioQuality   string   `toml:"audio_quality"`
	AllowedDomains []string `toml:"allowed_domains"`
}

// NgrokConfig contains ngrok tunnel configuration
type NgrokConfig struct {
	Enabled      bool   `toml:"enabled"`
	AuthToken    string `toml:"auth_token"`
	Domain       string `toml:"domain"`
	Region       string `toml:"region"`
	EnableAuth   bool   `toml:"enable_auth"`
	AuthProvider string `toml:"auth_provider"`
}

// DiscordConfig contains Discord Rich Presence configuration
type DiscordConfig struct {
	Enabled       bool   `toml:"enabled"`
	ApplicationID string `toml:"application_id"`
	LargeImageKey string `toml:"large_image_key"`
	SmallImageKey string `toml:"small_image_key"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:        "8080",
			Host:        "0.0.0.0",
			StaticDir:   "./static",
			EnableCORS:  true,
			ReadTimeout: 30,
		},
		Database: DatabaseConfig{
			Path:           "./staccato.db",
			MaxConnections: 10,
		},
		Music: MusicConfig{
			LibraryPath:      "./music",
			SupportedFormats: []string{".flac", ".mp3", ".wav", ".m4a"},
			WatchForChanges:  true,
			ScanOnStartup:    true,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			File:   "",
		},
		Downloader: DownloaderConfig{
			Enabled:        true,
			YtDlpPath:      "yt-dlp",
			MaxConcurrent:  2,
			AudioFormat:    "mp3",
			AudioQuality:   "0",
			AllowedDomains: []string{"youtube.com", "youtu.be", "soundcloud.com"},
		},
		Ngrok: NgrokConfig{
			Enabled:      false,
			AuthToken:    "",
			Domain:       "",
			Region:       "us",
			EnableAuth:   false,
			AuthProvider: "google",
		},
		Discord: DiscordConfig{
			Enabled:       false,
			ApplicationID: "1400564318365552771", // This should be replaced with your actual Discord app ID
			LargeImageKey: "staccato_logo",
			SmallImageKey: "music_note",
		},
	}
}

// LoadConfig loads configuration from a TOML file
func LoadConfig(configPath string) (*Config, error) {
	// Start with defaults
	cfg := DefaultConfig()

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Config file doesn't exist, create it with defaults
		if err := cfg.SaveToFile(configPath); err != nil {
			return nil, fmt.Errorf("failed to create default config file: %w", err)
		}
		fmt.Printf("Created default configuration file at: %s\n", configPath)
		return cfg, nil
	}

	// Load from file
	if _, err := toml.DecodeFile(configPath, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// SaveToFile saves the configuration to a TOML file
func (c *Config) SaveToFile(configPath string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create or open file
	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	// Write header comment
	header := `# Staccato Music Server Configuration
# This file contains all configuration options for the Staccato music streaming server.
# Edit the values below to customize your server settings.

`
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write config header: %w", err)
	}

	// Encode configuration to TOML
	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("failed to encode config to TOML: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.Port == "" {
		return fmt.Errorf("server port cannot be empty")
	}
	if c.Server.Host == "" {
		return fmt.Errorf("server host cannot be empty")
	}
	if c.Server.ReadTimeout < 0 {
		return fmt.Errorf("server read timeout must be positive")
	}

	// Validate database config
	if c.Database.Path == "" {
		return fmt.Errorf("database path cannot be empty")
	}
	if c.Database.MaxConnections < 1 {
		return fmt.Errorf("database max connections must be at least 1")
	}

	// Validate music config
	if c.Music.LibraryPath == "" {
		return fmt.Errorf("music library path cannot be empty")
	}
	if len(c.Music.SupportedFormats) == 0 {
		return fmt.Errorf("at least one supported audio format must be specified")
	}

	// Validate logging config
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLogLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.Logging.Level)
	}

	validLogFormats := map[string]bool{
		"text": true, "json": true,
	}
	if !validLogFormats[c.Logging.Format] {
		return fmt.Errorf("invalid log format: %s (must be text or json)", c.Logging.Format)
	}

	return nil
}

// GetAddress returns the full server address
func (c *Config) GetAddress() string {
	return c.Server.Host + ":" + c.Server.Port
}

// IsFormatSupported checks if an audio format is supported
func (c *Config) IsFormatSupported(format string) bool {
	for _, supported := range c.Music.SupportedFormats {
		if supported == format {
			return true
		}
	}
	return false
}
