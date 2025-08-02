// Audio player functions

// Setup keyboard shortcuts
function setupKeyboardShortcuts() {
    document.addEventListener('keydown', function(event) {
        // Only handle shortcuts when not typing in an input field
        if (event.target.tagName === 'INPUT' || event.target.tagName === 'TEXTAREA') {
            return;
        }
        
        switch(event.code) {
            case 'ArrowRight':
            case 'KeyN':
                event.preventDefault();
                playNextTrack();
                break;
            case 'ArrowLeft':
            case 'KeyP':
                event.preventDefault();
                playPreviousTrack();
                break;
            case 'Space':
                event.preventDefault();
                togglePlayPause();
                break;
        }
    });
}

// Toggle play/pause
function togglePlayPause() {
    const audioPlayer = document.getElementById('audioPlayer');
    const playPauseBtn = document.getElementById('playPauseBtn');
    const icon = playPauseBtn.querySelector('i');
    
    if (audioPlayer.src) {
        if (audioPlayer.paused) {
            audioPlayer.play();
            icon.className = 'nf nf-md-pause';
        } else {
            audioPlayer.pause();
            icon.className = 'nf nf-md-play';
        }
    }
}

// Toggle shuffle mode
function toggleShuffle() {
    isShuffled = !isShuffled;
    const shuffleBtn = document.getElementById('shuffleBtn');
    
    if (isShuffled) {
        shuffleBtn.classList.add('active');
        shuffleBtn.title = 'Shuffle On';
    } else {
        shuffleBtn.classList.remove('active');
        shuffleBtn.title = 'Shuffle Off';
    }
}

// Toggle repeat mode (off -> playlist -> track -> off)
function toggleRepeat() {
    repeatMode = (repeatMode + 1) % 3;
    const repeatBtn = document.getElementById('repeatBtn');
    const icon = repeatBtn.querySelector('i');
    
    switch (repeatMode) {
        case 0:
            repeatBtn.classList.remove('active');
            icon.className = 'nf nf-md-repeat';
            repeatBtn.title = 'Repeat Off';
            break;
        case 1:
            repeatBtn.classList.add('active');
            icon.className = 'nf nf-md-repeat';
            repeatBtn.title = 'Repeat Playlist';
            break;
        case 2:
            repeatBtn.classList.add('active');
            icon.className = 'nf nf-md-repeat_once';
            repeatBtn.title = 'Repeat Track';
            break;
    }
}

// Set volume from slider click
function setVolume(event) {
    const audioPlayer = document.getElementById('audioPlayer');
    const volumeSlider = document.getElementById('volumeSlider');
    const volumeFill = document.getElementById('volumeFill');
    const volumeBtn = document.getElementById('volumeBtn');
    const icon = volumeBtn.querySelector('i');
    
    const rect = volumeSlider.getBoundingClientRect();
    const clickX = event.clientX - rect.left;
    const percentage = Math.max(0, Math.min(1, clickX / rect.width));
    
    audioPlayer.volume = percentage;
    volumeFill.style.width = (percentage * 100) + '%';
    
    // Update volume icon
    if (percentage === 0) {
        icon.className = 'nf nf-md-volume_off';
        isMuted = true;
    } else {
        isMuted = false;
        if (percentage > 0.5) {
            icon.className = 'nf nf-md-volume_high';
        } else {
            icon.className = 'nf nf-md-volume_medium';
        }
    }
    
    lastVolume = percentage > 0 ? percentage : lastVolume;
}

// Toggle mute
function toggleMute() {
    const audioPlayer = document.getElementById('audioPlayer');
    const volumeBtn = document.getElementById('volumeBtn');
    const volumeFill = document.getElementById('volumeFill');
    const icon = volumeBtn.querySelector('i');
    
    if (isMuted) {
        audioPlayer.volume = lastVolume;
        volumeFill.style.width = (lastVolume * 100) + '%';
        icon.className = lastVolume > 0.5 ? 'nf nf-md-volume_high' : 'nf nf-md-volume_medium';
        isMuted = false;
    } else {
        lastVolume = audioPlayer.volume;
        audioPlayer.volume = 0;
        volumeFill.style.width = '0%';
        icon.className = 'nf nf-md-volume_off';
        isMuted = true;
    }
}

// Set volume from slider click
function setVolume(event) {
    const volumeSlider = event.currentTarget;
    const rect = volumeSlider.getBoundingClientRect();
    const percentage = (event.clientX - rect.left) / rect.width;
    const volume = Math.max(0, Math.min(1, percentage));
    
    const audioPlayer = document.getElementById('audioPlayer');
    const volumeFill = document.getElementById('volumeFill');
    const volumeBtn = document.getElementById('volumeBtn');
    const icon = volumeBtn.querySelector('i');
    
    audioPlayer.volume = volume;
    volumeFill.style.width = (volume * 100) + '%';
    
    if (volume === 0) {
        icon.className = 'nf nf-md-volume_off';
        isMuted = true;
    } else {
        icon.className = volume > 0.5 ? 'nf nf-md-volume_high' : 'nf nf-md-volume_medium';
        isMuted = false;
        lastVolume = volume;
    }
}

// Seek to position in track
function seekToPosition(event) {
    const progressBar = event.currentTarget;
    const rect = progressBar.getBoundingClientRect();
    const percentage = (event.clientX - rect.left) / rect.width;
    const audioPlayer = document.getElementById('audioPlayer');
    
    if (audioPlayer.duration) {
        audioPlayer.currentTime = audioPlayer.duration * percentage;
    }
}

// Format time for display
function formatTime(seconds) {
    if (isNaN(seconds)) return '0:00';
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = Math.floor(seconds % 60);
    return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`;
}

// Update progress bar and time display
function updateProgress() {
    const audioPlayer = document.getElementById('audioPlayer');
    const progressFill = document.getElementById('progressFill');
    const currentTimeSpan = document.getElementById('currentTime');
    const totalTimeSpan = document.getElementById('totalTime');
    
    if (audioPlayer.duration) {
        const percentage = (audioPlayer.currentTime / audioPlayer.duration) * 100;
        progressFill.style.width = percentage + '%';
        currentTimeSpan.textContent = formatTime(audioPlayer.currentTime);
        totalTimeSpan.textContent = formatTime(audioPlayer.duration);
    }
}

// Seek to position when progress bar is clicked
function seekToPosition(event) {
    const audioPlayer = document.getElementById('audioPlayer');
    const progressBar = document.getElementById('progressBar');
    
    if (audioPlayer.duration) {
        const rect = progressBar.getBoundingClientRect();
        const clickX = event.clientX - rect.left;
        const percentage = clickX / rect.width;
        const newTime = percentage * audioPlayer.duration;
        
        audioPlayer.currentTime = newTime;
        updateProgress();
    }
}

// Play next track
function playNextTrack() {
    if (tracks.length === 0 || currentTrackIndex === -1) {
        console.log('No tracks to play next');
        return;
    }
    
    let nextIndex;
    
    if (isShuffled) {
        // Random next track
        do {
            nextIndex = Math.floor(Math.random() * tracks.length);
        } while (nextIndex === currentTrackIndex && tracks.length > 1);
    } else {
        // Sequential next track
        nextIndex = currentTrackIndex + 1;
        
        if (nextIndex >= tracks.length) {
            if (repeatMode === 1) { // Repeat playlist
                nextIndex = 0;
            } else {
                console.log('End of playlist reached');
                return;
            }
        }
    }
    
    if (nextIndex < tracks.length && nextIndex >= 0) {
        playTrack(tracks[nextIndex].id);
    }
}

// Play previous track
function playPreviousTrack() {
    if (tracks.length === 0 || currentTrackIndex === -1) {
        console.log('No tracks to play previous');
        return;
    }
    
    let prevIndex = currentTrackIndex - 1;
    
    if (prevIndex < 0) {
        if (repeatMode === 1) { // Repeat playlist
            prevIndex = tracks.length - 1;
        } else {
            console.log('At beginning of playlist');
            return;
        }
    }
    
    if (prevIndex < tracks.length && prevIndex >= 0) {
        playTrack(tracks[prevIndex].id);
    }
}

// Setup audio player event listeners
document.addEventListener('DOMContentLoaded', function() {
    const audioPlayer = document.getElementById('audioPlayer');
    if (audioPlayer) {
        audioPlayer.addEventListener('timeupdate', updateProgress);
        audioPlayer.addEventListener('loadedmetadata', updateProgress);
        
        // Handle track ending
        audioPlayer.addEventListener('ended', function() {
            if (repeatMode === 2) { // Repeat track
                audioPlayer.currentTime = 0;
                audioPlayer.play();
            } else {
                playNextTrack();
            }
        });
        
        // Handle audio errors
        audioPlayer.addEventListener('error', function(e) {
            console.error('Audio player error:', e);
            const errorMsg = 'Error playing audio. Please try another track.';
            // You could show this error to the user in the UI
        });
    }
});
