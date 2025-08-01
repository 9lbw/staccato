// Session management
async function createSession() {
    try {
        const deviceName = getDeviceName();
        const response = await fetch('/api/sessions/create', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ deviceName: deviceName })
        });
        
        if (response.ok) {
            const data = await response.json();
            sessionId = data.sessionId;
            isActiveSession = data.isActive;
            console.log(`Session created: ${sessionId} (${deviceName})${isActiveSession ? ' - ACTIVE' : ''}`);
            
            // Update UI to show session status
            updateSessionStatus();
        } else {
            console.error('Failed to create session');
        }
    } catch (error) {
        console.error('Error creating session:', error);
    }
}

// Get device name based on user agent
function getDeviceName() {
    const ua = navigator.userAgent.toLowerCase();
    
    if (ua.includes('mobile') || ua.includes('android')) {
        if (ua.includes('android')) return 'Android Device';
        return 'Mobile Device';
    }
    if (ua.includes('iphone')) return 'iPhone';
    if (ua.includes('ipad')) return 'iPad';
    if (ua.includes('mac') || ua.includes('macintosh')) return 'Mac';
    if (ua.includes('windows')) return 'Windows PC';
    if (ua.includes('linux')) return 'Linux PC';
    
    return 'Web Browser';
}

// Update session status in UI
function updateSessionStatus() {
    // You can add a visual indicator here
    const statusElement = document.getElementById('session-status');
    if (statusElement) {
        statusElement.textContent = isActiveSession ? 'CONTROLLING DISCORD' : 'BACKGROUND SESSION';
        statusElement.className = isActiveSession ? 'session-active' : 'session-inactive';
    }
}

// Check if other sessions are active
async function checkSessionStatus() {
    try {
        const response = await fetch('/api/sessions');
        if (response.ok) {
            const data = await response.json();
            const wasActive = isActiveSession;
            isActiveSession = data.activeSessionId === sessionId;
            
            if (wasActive !== isActiveSession) {
                console.log(`Session status changed: ${isActiveSession ? 'ACTIVE' : 'INACTIVE'}`);
                updateSessionStatus();
            }
        }
    } catch (error) {
        console.error('Error checking session status:', error);
    }
}

// Check for session events and handle cross-session communication using Server-Sent Events
let eventSource = null;

function startSessionEventStream() {
    if (!sessionId) return;
    
    // Close existing connection
    if (eventSource) {
        eventSource.close();
    }
    
    console.log('Starting SSE session event stream for session:', sessionId);
    eventSource = new EventSource(`/api/sessions/stream?sessionId=${sessionId}`);
    
    eventSource.onmessage = function(event) {
        try {
            const data = JSON.parse(event.data);
            
            if (data.type === 'pauseCommand' && data.shouldPause) {
                const audioPlayer = document.getElementById('audioPlayer');
                
                if (audioPlayer && !audioPlayer.paused) {
                    audioPlayer.pause();
                    
                    // Update play button
                    const playPauseBtn = document.getElementById('playPauseBtn');
                    if (playPauseBtn) {
                        const icon = playPauseBtn.querySelector('i');
                        if (icon) icon.className = 'nf nf-md-play';
                    }
                    
                    // Show notification
                    showNotification(`Playback paused: ${data.pauseReason}`);
                    
                    // Update Discord RPC
                    updateDiscordRPC();
                }
            } else if (data.type === 'sessionState') {
                // Update session status
                const wasActive = isActiveSession;
                isActiveSession = data.isActive;
                
                if (wasActive !== isActiveSession) {
                    updateSessionStatus();
                }
            }
        } catch (error) {
            console.error('Error parsing SSE event:', error);
        }
    };
    
    eventSource.onerror = function(event) {
        console.error('SSE connection error:', event);
        // Reconnect after a delay
        setTimeout(() => {
            if (sessionId) {
                startSessionEventStream();
            }
        }, 5000);
    };
    
    eventSource.onopen = function(event) {
        console.log('SSE connection opened for session:', sessionId);
    };
}

// Update Discord RPC with current state (session-aware)
async function updateDiscordRPC() {
    if (!sessionId) {
        console.warn('No session ID available for Discord RPC update');
        return;
    }
    
    // Debounce rapid updates
    clearTimeout(discordUpdateTimeout);
    discordUpdateTimeout = setTimeout(async () => {
        try {
            const audioPlayer = document.getElementById('audioPlayer');
            const state = {
                sessionId: sessionId,
                isPlaying: !audioPlayer.paused && !!audioPlayer.src,
                currentTime: Math.floor(audioPlayer.currentTime || 0),
                totalDuration: Math.floor(audioPlayer.duration || 0),
                volume: audioPlayer.volume || 1.0,
                isMuted: isMuted,
                isShuffled: isShuffled,
                repeatMode: repeatMode
            };
            
            // Include track ID if we have one
            if (currentTrackId !== null) {
                state.trackId = currentTrackId;
            }
            
            console.log('Sending session Discord RPC update:', state);
            
            const response = await fetch('/api/sessions/update', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(state)
            });
            
            if (response.ok) {
                const data = await response.json();
                isActiveSession = data.isActive;
                updateSessionStatus();
                
                // If we just started playing, immediately check for pause commands on other sessions
                if (state.isPlaying && state.trackId) {
                    // Trigger immediate session event check for all sessions
                    setTimeout(() => {
                        // This will cause other sessions to check for pause commands immediately
                        // The server will have already processed our play command and set WasPaused flags
                    }, 50); // Small delay to ensure server has processed our update
                }
            } else {
                console.error('Session Discord RPC update failed:', response.status, response.statusText);
            }
        } catch (error) {
            console.error('Failed to update session Discord RPC:', error);
        }
    }, 500); // Wait 500ms before sending update
}
