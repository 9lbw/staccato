package player

import (
	"sync"
	"time"

	"staccato/pkg/models"
)

// State represents the current player state
type State struct {
	Track         *models.Track `json:"track,omitempty"`
	IsPlaying     bool          `json:"isPlaying"`
	CurrentTime   int           `json:"currentTime"`   // in seconds
	TotalDuration int           `json:"totalDuration"` // in seconds
	Volume        float64       `json:"volume"`        // 0.0 to 1.0
	IsMuted       bool          `json:"isMuted"`
	IsShuffled    bool          `json:"isShuffled"`
	RepeatMode    int           `json:"repeatMode"` // 0 = off, 1 = playlist, 2 = track
	UpdatedAt     time.Time     `json:"updatedAt"`
}

// StateManager manages the player state and notifies listeners
type StateManager struct {
	state     *State
	mutex     sync.RWMutex
	listeners []chan *State
}

// NewStateManager creates a new player state manager
func NewStateManager() *StateManager {
	return &StateManager{
		state: &State{
			Volume:    1.0,
			UpdatedAt: time.Now(),
		},
		listeners: make([]chan *State, 0),
	}
}

// GetState returns the current player state (thread-safe)
func (sm *StateManager) GetState() *State {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	// Create a copy to avoid race conditions
	stateCopy := *sm.state
	return &stateCopy
}

// UpdateTrack updates the currently playing track
func (sm *StateManager) UpdateTrack(track *models.Track) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.state.Track = track
	sm.state.UpdatedAt = time.Now()
	sm.notifyListeners()
}

// UpdatePlaybackState updates playback state (playing/paused)
func (sm *StateManager) UpdatePlaybackState(isPlaying bool) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.state.IsPlaying = isPlaying
	sm.state.UpdatedAt = time.Now()
	sm.notifyListeners()
}

// UpdateTime updates current playback time and duration
func (sm *StateManager) UpdateTime(currentTime, totalDuration int) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.state.CurrentTime = currentTime
	sm.state.TotalDuration = totalDuration
	sm.state.UpdatedAt = time.Now()
	sm.notifyListeners()
}

// UpdateVolume updates volume and mute state
func (sm *StateManager) UpdateVolume(volume float64, isMuted bool) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.state.Volume = volume
	sm.state.IsMuted = isMuted
	sm.state.UpdatedAt = time.Now()
	sm.notifyListeners()
}

// UpdateSettings updates player settings (shuffle, repeat)
func (sm *StateManager) UpdateSettings(isShuffled bool, repeatMode int) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.state.IsShuffled = isShuffled
	sm.state.RepeatMode = repeatMode
	sm.state.UpdatedAt = time.Now()
	sm.notifyListeners()
}

// ClearTrack clears the current track (when playback stops)
func (sm *StateManager) ClearTrack() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.state.Track = nil
	sm.state.IsPlaying = false
	sm.state.CurrentTime = 0
	sm.state.TotalDuration = 0
	sm.state.UpdatedAt = time.Now()
	sm.notifyListeners()
}

// Subscribe adds a listener for state changes
func (sm *StateManager) Subscribe() <-chan *State {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	ch := make(chan *State, 10) // Buffered channel to prevent blocking
	sm.listeners = append(sm.listeners, ch)
	return ch
}

// Unsubscribe removes a listener (call this when done to prevent memory leaks)
func (sm *StateManager) Unsubscribe(ch <-chan *State) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	for i, listener := range sm.listeners {
		if listener == ch {
			close(listener)
			sm.listeners = append(sm.listeners[:i], sm.listeners[i+1:]...)
			break
		}
	}
}

// notifyListeners sends state updates to all subscribers (must be called with lock held)
func (sm *StateManager) notifyListeners() {
	stateCopy := *sm.state
	for i, listener := range sm.listeners {
		select {
		case listener <- &stateCopy:
			// Successfully sent
		default:
			// Channel is full or closed, remove it
			close(listener)
			sm.listeners = append(sm.listeners[:i], sm.listeners[i+1:]...)
		}
	}
}
