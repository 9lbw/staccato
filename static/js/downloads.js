// Download management functions

// Handle download input enter key
function handleDownloadInput(event) {
    if (event.key === 'Enter') {
        startDownload();
    }
}

// Start download
async function startDownload() {
    const urlInput = document.getElementById('downloadUrl');
    const url = urlInput.value.trim();
    
    if (!url) {
        alert('Please enter a URL');
        return;
    }
    
    const downloadBtn = document.querySelector('.btn-download');
    downloadBtn.disabled = true;
    downloadBtn.textContent = 'Starting...';
    
    try {
        const response = await fetch('/api/download', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url: url })
        });
        
        if (response.ok) {
            const result = await response.json();
            urlInput.value = '';
            showNotification('Download started!');
            refreshDownloadJobs();
        } else {
            const error = await response.text();
            alert('Failed to start download: ' + error);
        }
    } catch (error) {
        console.error('Error starting download:', error);
        alert('Error starting download');
    } finally {
        downloadBtn.disabled = false;
        downloadBtn.textContent = 'Download';
    }
}

// Refresh download jobs
async function refreshDownloadJobs() {
    try {
        const response = await fetch('/api/download/jobs');
        if (response.ok) {
            const jobs = await response.json();
            displayDownloadJobs(jobs);
            updateDownloadStats(jobs);
        } else {
            console.error('Failed to load download jobs');
        }
    } catch (error) {
        console.error('Error loading download jobs:', error);
    }
}

// Display download jobs
function displayDownloadJobs(jobs) {
    const jobsContainer = document.getElementById('downloadJobs');
    if (!jobsContainer) return;
    
    if (jobs.length === 0) {
        jobsContainer.innerHTML = '<div class="no-downloads">No downloads yet. Add a URL above to get started!</div>';
        return;
    }
    
    jobsContainer.innerHTML = jobs.map(job => `
        <div class="download-job">
            <div class="job-header">
                <div class="job-title">${escapeHtml(job.title || job.url)}</div>
                <div class="job-status status-${job.status}">${job.status}</div>
            </div>
            <div class="job-url">${escapeHtml(job.url)}</div>
            ${job.progress !== undefined ? `
                <div class="job-progress-container" style="display: block;">
                    <div class="job-progress-bar">
                        <div class="job-progress-fill" style="width: ${job.progress}%"></div>
                    </div>
                    <div class="job-progress-text">${job.progress}% complete</div>
                </div>
            ` : ''}
            ${job.error ? `<div class="job-error">${escapeHtml(job.error)}</div>` : ''}
        </div>
    `).join('');
}

// Update download statistics
function updateDownloadStats(jobs) {
    const totalElement = document.getElementById('totalDownloads');
    const activeElement = document.getElementById('activeDownloads');
    const completedElement = document.getElementById('completedDownloads');
    const failedElement = document.getElementById('failedDownloads');
    
    if (totalElement) totalElement.textContent = jobs.length;
    if (activeElement) activeElement.textContent = jobs.filter(j => j.status === 'downloading' || j.status === 'pending').length;
    if (completedElement) completedElement.textContent = jobs.filter(j => j.status === 'completed').length;
    if (failedElement) failedElement.textContent = jobs.filter(j => j.status === 'failed').length;
}

// Auto-refresh download jobs when on downloads section
let downloadRefreshInterval;

function startDownloadAutoRefresh() {
    if (downloadRefreshInterval) {
        clearInterval(downloadRefreshInterval);
    }
    
    downloadRefreshInterval = setInterval(() => {
        if (currentSection === 'downloads') {
            refreshDownloadJobs();
        }
    }, 2000); // Refresh every 2 seconds
}

function stopDownloadAutoRefresh() {
    if (downloadRefreshInterval) {
        clearInterval(downloadRefreshInterval);
        downloadRefreshInterval = null;
    }
}

// Start auto-refresh when showing downloads section
const originalShowSection = showSection;
showSection = function(section) {
    originalShowSection(section);
    
    if (section === 'downloads') {
        refreshDownloadJobs();
        startDownloadAutoRefresh();
    } else {
        stopDownloadAutoRefresh();
    }
};

// Initialize downloads when page loads
document.addEventListener('DOMContentLoaded', function() {
    if (currentSection === 'downloads') {
        refreshDownloadJobs();
        startDownloadAutoRefresh();
    }
});
