// Global variables
let tracks = [];
let playlists = [];
let currentSection = 'library';
let currentPlaylistId = null;
let selectedTrackId = null;
let lastTrackCount = 0;
let currentTrackList = []; // Stores the current list of tracks being played
let currentTrackIndex = -1; // Index of currently playing track in the list
let currentTrackId = null; // ID of currently playing track
let sessionId = null; // Current session ID
let isActiveSession = false; // Whether this is the active session

// Player state variables
let isShuffled = false;
let repeatMode = 0; // 0 = off, 1 = playlist, 2 = track
let isMuted = false;
let lastVolume = 1;

// Discord RPC integration
let discordUpdateTimeout;

// Mobile sidebar toggle
function toggleSidebar() {
    const sidebar = document.getElementById('sidebar');
    sidebar.classList.toggle('open');
}

// Close sidebar when clicking outside on mobile
document.addEventListener('click', function(event) {
    const sidebar = document.getElementById('sidebar');
    const menuBtn = document.querySelector('.mobile-menu-btn');
    
    if (window.innerWidth <= 768 && 
        sidebar.classList.contains('open') && 
        !sidebar.contains(event.target) && 
        !menuBtn.contains(event.target)) {
        sidebar.classList.remove('open');
    }
});

// Close sidebar when window is resized to desktop
window.addEventListener('resize', function() {
    const sidebar = document.getElementById('sidebar');
    if (window.innerWidth > 768) {
        sidebar.classList.remove('open');
    }
});

// Initialize the app
async function init() {
    // Create session first
    await createSession();
    
    await loadTracks();
    await loadPlaylists();
    setupKeyboardShortcuts();
    
    // Start checking session status periodically
    setInterval(checkSessionStatus, 10000); // Check every 10 seconds
    
    // Start real-time session event stream instead of polling
    startSessionEventStream();
}

// Show notification
function showNotification(message) {
    // Simple notification in console for now
    console.log(`[NOTIFICATION] ${message}`);
    
    // Update stats to show the notification
    const stats = document.getElementById('stats');
    const originalText = stats.textContent;
    stats.textContent = message;
    stats.style.color = '#ffffff';
    
    setTimeout(() => {
        stats.style.color = '#666';
    }, 3000);
}

// Update stats display
function updateStats() {
    const stats = document.getElementById('stats');
    stats.textContent = `${tracks.length} TRACKS INDEXED`;
}

// Initialize when page loads
document.addEventListener('DOMContentLoaded', init);
