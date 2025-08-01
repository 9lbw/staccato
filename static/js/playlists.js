// Playlist management functions

// Show create playlist modal
function showCreatePlaylistModal() {
    const modal = document.getElementById('createPlaylistModal');
    if (modal) {
        modal.style.display = 'block';
        // Clear form
        document.getElementById('playlistName').value = '';
        document.getElementById('playlistDescription').value = '';
    }
}

// Close create playlist modal
function closeCreatePlaylistModal() {
    const modal = document.getElementById('createPlaylistModal');
    if (modal) {
        modal.style.display = 'none';
    }
}

// Create playlist
async function createPlaylist(event) {
    event.preventDefault();
    
    const name = document.getElementById('playlistName').value;
    const description = document.getElementById('playlistDescription').value;
    
    if (!name.trim()) {
        alert('Please enter a playlist name');
        return;
    }
    
    try {
        const response = await fetch('/api/playlists', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: name.trim(), description: description.trim() })
        });
        
        if (response.ok) {
            closeCreatePlaylistModal();
            await loadPlaylists(); // Reload playlists
            showNotification('Playlist created successfully!');
        } else {
            const error = await response.text();
            alert('Failed to create playlist: ' + error);
        }
    } catch (error) {
        console.error('Error creating playlist:', error);
        alert('Error creating playlist');
    }
}

// Show add to playlist modal
function showAddToPlaylistModal(trackId) {
    selectedTrackId = trackId;
    const modal = document.getElementById('addToPlaylistModal');
    const optionsContainer = document.getElementById('playlistOptions');
    
    if (!modal || !optionsContainer) return;
    
    if (playlists.length === 0) {
        optionsContainer.innerHTML = '<div class="loading">No playlists available. Create a playlist first.</div>';
    } else {
        optionsContainer.innerHTML = playlists.map(playlist => `
            <div class="playlist-item" onclick="addTrackToPlaylist(${trackId}, ${playlist.id})">
                <div class="playlist-meta">
                    <div class="playlist-name">${escapeHtml(playlist.name)}</div>
                    <div class="playlist-info">${playlist.track_count || 0} tracks</div>
                </div>
            </div>
        `).join('');
    }
    
    modal.style.display = 'block';
}

// Close add to playlist modal
function closeAddToPlaylistModal() {
    const modal = document.getElementById('addToPlaylistModal');
    if (modal) {
        modal.style.display = 'none';
    }
    selectedTrackId = null;
}

// Add track to playlist
async function addTrackToPlaylist(trackId, playlistId) {
    try {
        const response = await fetch(`/api/playlists/${playlistId}/tracks`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ trackId: trackId })
        });
        
        if (response.ok) {
            closeAddToPlaylistModal();
            await loadPlaylists(); // Reload to update track counts
            showNotification('Track added to playlist!');
        } else {
            const error = await response.text();
            alert('Failed to add track to playlist: ' + error);
        }
    } catch (error) {
        console.error('Error adding track to playlist:', error);
        alert('Error adding track to playlist');
    }
}

// Show playlist details
async function showPlaylist(playlistId) {
    currentPlaylistId = playlistId;
    
    try {
        const response = await fetch(`/api/playlists/${playlistId}/tracks`);
        if (response.ok) {
            const playlistTracks = await response.json();
            const playlist = playlists.find(p => p.id === playlistId);
            
            if (playlist) {
                document.getElementById('playlist-detail-title').textContent = playlist.name;
            }
            
            const tracksContainer = document.getElementById('playlist-tracks');
            if (playlistTracks.length === 0) {
                tracksContainer.innerHTML = '<div class="loading">No tracks in this playlist</div>';
            } else {
                tracksContainer.innerHTML = playlistTracks.map(track => `
                    <div class="track-item${currentTrackId === track.id ? ' playing' : ''}" onclick="playTrack(${track.id})">
                        <div class="track-info">
                            <div class="track-title">${escapeHtml(track.title)}</div>
                            <div class="track-artist">${escapeHtml(track.artist)}</div>
                        </div>
                        <div class="track-actions">
                            <button class="play-btn" onclick="event.stopPropagation(); playTrack(${track.id})">PLAY</button>
                            <button class="btn btn-danger" onclick="event.stopPropagation(); removeTrackFromPlaylist(${track.id}, ${playlistId})">REMOVE</button>
                        </div>
                    </div>
                `).join('');
            }
            
            showSection('playlist-detail');
        } else {
            console.error('Failed to load playlist tracks');
        }
    } catch (error) {
        console.error('Error loading playlist tracks:', error);
    }
}

// Remove track from playlist
async function removeTrackFromPlaylist(trackId, playlistId) {
    if (!confirm('Remove this track from the playlist?')) {
        return;
    }
    
    try {
        const response = await fetch(`/api/playlists/${playlistId}/tracks/${trackId}`, {
            method: 'DELETE'
        });
        
        if (response.ok) {
            await showPlaylist(playlistId); // Reload playlist
            await loadPlaylists(); // Update playlist counts
            showNotification('Track removed from playlist!');
        } else {
            const error = await response.text();
            alert('Failed to remove track from playlist: ' + error);
        }
    } catch (error) {
        console.error('Error removing track from playlist:', error);
        alert('Error removing track from playlist');
    }
}

// Delete playlist
async function deletePlaylist(playlistId) {
    const playlist = playlists.find(p => p.id === playlistId);
    const playlistName = playlist ? playlist.name : 'this playlist';
    
    if (!confirm(`Delete "${playlistName}"? This cannot be undone.`)) {
        return;
    }
    
    try {
        const response = await fetch(`/api/playlists/${playlistId}`, {
            method: 'DELETE'
        });
        
        if (response.ok) {
            await loadPlaylists(); // Reload playlists
            showNotification('Playlist deleted!');
            
            // If we're currently viewing this playlist, go back to playlists view
            if (currentPlaylistId === playlistId) {
                showSection('playlists');
            }
        } else {
            const error = await response.text();
            alert('Failed to delete playlist: ' + error);
        }
    } catch (error) {
        console.error('Error deleting playlist:', error);
        alert('Error deleting playlist');
    }
}

// Close modals when clicking outside
window.addEventListener('click', function(event) {
    const createModal = document.getElementById('createPlaylistModal');
    const addModal = document.getElementById('addToPlaylistModal');
    
    if (event.target === createModal) {
        closeCreatePlaylistModal();
    }
    
    if (event.target === addModal) {
        closeAddToPlaylistModal();
    }
});
