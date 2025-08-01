package discord

import (
	"fmt"
	"log"
	"time"

	"staccato/internal/config"
	"staccato/pkg/models"

	"github.com/hugolgst/rich-go/client"
)

// RPCService handles Discord Rich Presence functionality
type RPCService struct {
	config    *config.DiscordConfig
	enabled   bool
	connected bool
}

// NewRPCService creates a new Discord RPC service
func NewRPCService(cfg *config.DiscordConfig) *RPCService {
	return &RPCService{
		config:  cfg,
		enabled: cfg.Enabled,
	}
}

// Connect initializes the Discord RPC connection
func (d *RPCService) Connect() error {
	if !d.enabled {
		return nil
	}

	if d.connected {
		return nil
	}

	err := client.Login(d.config.ApplicationID)
	if err != nil {
		return fmt.Errorf("failed to connect to Discord: %w", err)
	}

	d.connected = true
	log.Println("✅ Connected to Discord RPC")

	// Set initial idle state
	d.SetIdle()
	return nil
}

// Disconnect closes the Discord RPC connection
func (d *RPCService) Disconnect() {
	if !d.enabled || !d.connected {
		return
	}

	client.Logout()
	d.connected = false
	log.Println("🔌 Disconnected from Discord RPC")
}

// UpdateNowPlaying updates Discord status with currently playing track
func (d *RPCService) UpdateNowPlaying(track *models.Track, isPlaying bool, currentTime, totalDuration int) error {
	if !d.enabled || !d.connected {
		return nil
	}

	smallImageKey := "pause"
	smallImageText := "Paused"

	if isPlaying {
		smallImageKey = "play"
		smallImageText = "Playing"
	}

	// Calculate timestamps for progress bar
	now := time.Now()
	var startTimestamp *time.Time
	var endTimestamp *time.Time

	if isPlaying && totalDuration > 0 {
		// Set start time based on current position
		startTime := now.Add(-time.Duration(currentTime) * time.Second)
		startTimestamp = &startTime

		// Set end time based on total duration
		endTime := now.Add(time.Duration(totalDuration-currentTime) * time.Second)
		endTimestamp = &endTime
	}

	activity := client.Activity{
		Details:    fmt.Sprintf("%s", track.Title),
		State:      fmt.Sprintf("by %s", track.Artist),
		LargeImage: d.config.LargeImageKey,
		LargeText:  "Music Server",
		SmallImage: smallImageKey,
		SmallText:  smallImageText,
	}

	// Add timestamps if playing
	if startTimestamp != nil {
		activity.Timestamps = &client.Timestamps{
			Start: startTimestamp,
			End:   endTimestamp,
		}
	}

	// Add album info if available
	if track.Album != "" {
		activity.State = fmt.Sprintf("by %s • %s", track.Artist, track.Album)
	}

	err := client.SetActivity(activity)
	if err != nil {
		return fmt.Errorf("failed to update Discord activity: %w", err)
	}

	return nil
}

// SetIdle sets Discord status to idle/not playing
func (d *RPCService) SetIdle() error {
	if !d.enabled || !d.connected {
		return nil
	}

	activity := client.Activity{
		Details:    "Browsing Music Library",
		State:      "Not playing",
		LargeImage: d.config.LargeImageKey,
		LargeText:  "Music Server",
		SmallImage: "idle",
		SmallText:  "Idle",
	}

	err := client.SetActivity(activity)
	if err != nil {
		return fmt.Errorf("failed to set idle Discord activity: %w", err)
	}

	return nil
}

// IsEnabled returns whether Discord RPC is enabled
func (d *RPCService) IsEnabled() bool {
	return d.enabled
}

// IsConnected returns whether Discord RPC is connected
func (d *RPCService) IsConnected() bool {
	return d.connected
}

// Enable enables Discord RPC (requires reconnection)
func (d *RPCService) Enable() {
	d.enabled = true
	d.config.Enabled = true
}

// Disable disables Discord RPC and disconnects
func (d *RPCService) Disable() {
	d.enabled = false
	d.config.Enabled = false
	d.Disconnect()
}
