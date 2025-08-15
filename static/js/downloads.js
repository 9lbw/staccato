// Download management functions

let selectedFiles = [];

// Handle download input enter key
function handleDownloadInput(event) {
    if (event.key === 'Enter') {
        startDownload();
    }
}

// Handle file selection for upload
function handleFileSelection(event) {
    const files = Array.from(event.target.files);
    const uploadBtn = document.querySelector('.btn-upload');
    const selectedFilesContainer = document.getElementById('selectedFiles');
    const selectedFilesList = document.getElementById('selectedFilesList');
    
    selectedFiles = files;
    
    if (files.length > 0) {
        // Show selected files
        selectedFilesContainer.style.display = 'block';
        selectedFilesList.innerHTML = files.map((file, index) => `
            <div class="selected-file-item">
                <span class="selected-file-name">${escapeHtml(file.name)}</span>
                <span class="selected-file-size">${formatFileSize(file.size)}</span>
                <button class="selected-file-remove" onclick="removeSelectedFile(${index})">×</button>
            </div>
        `).join('');
        
        // Enable upload button
        uploadBtn.disabled = false;
    } else {
        // Hide selected files and disable upload button
        selectedFilesContainer.style.display = 'none';
        uploadBtn.disabled = true;
        selectedFiles = [];
    }
}

// Remove a selected file
function removeSelectedFile(index) {
    selectedFiles.splice(index, 1);
    
    // Update file input
    const fileInput = document.getElementById('uploadFile');
    const dt = new DataTransfer();
    selectedFiles.forEach(file => dt.items.add(file));
    fileInput.files = dt.files;
    
    // Trigger change event to update UI
    handleFileSelection({ target: fileInput });
}

// Format file size for display
function formatFileSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

// Start file upload
async function startUpload() {
    if (selectedFiles.length === 0) {
        alert('Please select files to upload');
        return;
    }
    
    const uploadBtn = document.querySelector('.btn-upload');
    const uploadForm = document.querySelector('.upload-form');
    
    uploadBtn.disabled = true;
    uploadBtn.textContent = 'Uploading...';
    
    // Create progress container
    let progressContainer = document.getElementById('uploadProgress');
    if (!progressContainer) {
        progressContainer = document.createElement('div');
        progressContainer.id = 'uploadProgress';
        progressContainer.className = 'upload-progress';
        progressContainer.innerHTML = `
            <div class="upload-progress-bar">
                <div class="upload-progress-fill" id="uploadProgressFill"></div>
            </div>
            <div class="upload-progress-text" id="uploadProgressText">Preparing upload...</div>
        `;
        uploadForm.appendChild(progressContainer);
    }
    
    const progressFill = document.getElementById('uploadProgressFill');
    const progressText = document.getElementById('uploadProgressText');
    
    let successCount = 0;
    let errorCount = 0;
    
    for (let i = 0; i < selectedFiles.length; i++) {
        const file = selectedFiles[i];
        const formData = new FormData();
        formData.append('file', file);
        
        try {
            progressText.textContent = `Uploading ${file.name} (${i + 1}/${selectedFiles.length})...`;
            progressFill.style.width = `${(i / selectedFiles.length) * 100}%`;
            
            const response = await fetch('/api/tracks/upload', {
                method: 'POST',
                body: formData
            });
            
            if (response.ok) {
                successCount++;
                console.log(`Successfully uploaded: ${file.name}`);
            } else {
                errorCount++;
                const errorText = await response.text();
                console.error(`Failed to upload ${file.name}:`, errorText);
            }
        } catch (error) {
            errorCount++;
            console.error(`Error uploading ${file.name}:`, error);
        }
    }
    
    // Update final progress
    progressFill.style.width = '100%';
    if (errorCount === 0) {
        progressText.textContent = `Successfully uploaded ${successCount} file${successCount !== 1 ? 's' : ''}!`;
        progressContainer.style.borderColor = 'rgba(25, 135, 84, 0.3)';
        
        // Show success message
        showNotification(`Successfully uploaded ${successCount} file${successCount !== 1 ? 's' : ''}!`);
        
        // Refresh library if we're currently viewing it
        if (currentSection === 'library') {
            loadTracks();
        }
    } else {
        progressText.textContent = `Upload completed: ${successCount} successful, ${errorCount} failed`;
        progressContainer.style.borderColor = 'rgba(220, 53, 69, 0.3)';
        
        showNotification(`Upload completed with ${errorCount} error${errorCount !== 1 ? 's' : ''}`);
    }
    
    // Reset form after a delay
    setTimeout(() => {
        uploadBtn.disabled = false;
        uploadBtn.textContent = 'Upload';
        
        // Clear file selection
        const fileInput = document.getElementById('uploadFile');
        fileInput.value = '';
        selectedFiles = [];
        document.getElementById('selectedFiles').style.display = 'none';
        
        // Remove progress container
        if (progressContainer.parentNode) {
            progressContainer.parentNode.removeChild(progressContainer);
        }
    }, 3000);
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
    const response = await fetch('/api/downloads');
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
    
    jobsContainer.innerHTML = jobs.map(job => {
        const eta = job.eta_seconds && job.status !== 'completed' && job.status !== 'failed'
            ? formatETA(job.eta_seconds) : '';
        const speed = job.speed ? job.speed : '';
        const metaLine = (speed || eta) ? `<div class="job-meta-line">${speed ? 'Speed: ' + escapeHtml(speed) : ''} ${eta ? 'ETA: ' + eta : ''}</div>` : '';
        return `
        <div class="download-job">
            <div class="job-header">
                <div class="job-title">${escapeHtml(job.title || job.url)}</div>
                <div class="job-status status-${job.status}">${job.status}</div>
            </div>
            <div class="job-url">${escapeHtml(job.url)}</div>
            ${metaLine}
            ${job.progress !== undefined ? `
                <div class="job-progress-container" style="display: block;">
                    <div class="job-progress-bar">
                        <div class="job-progress-fill" style="width: ${job.progress}%"></div>
                    </div>
                    <div class="job-progress-text">${job.progress}% complete</div>
                </div>
            ` : ''}
            ${job.error ? `<div class="job-error">${escapeHtml(job.error)}</div>` : ''}
        </div>`;
    }).join('');
}

function formatETA(seconds) {
    if (!seconds || seconds < 0) return '';
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = seconds % 60;
    if (h > 0) return `${h}h ${m}m ${s}s`;
    if (m > 0) return `${m}m ${s}s`;
    return `${s}s`;
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
    checkUploadAvailability();
    
    if (currentSection === 'downloads') {
        refreshDownloadJobs();
        startDownloadAutoRefresh();
    }
});

// Check if upload functionality should be available
async function checkUploadAvailability() {
    try {
        const response = await fetch('/api/config');
        if (response.ok) {
            const config = await response.json();
            const uploadForm = document.getElementById('uploadForm');
            
            if (uploadForm) {
                if (config.auth.enabled && config.auth.user_folders && config.auth.allow_uploads) {
                    // Show upload form
                    uploadForm.style.display = 'block';
                } else {
                    // Hide upload form
                    uploadForm.style.display = 'none';
                }
            }
        } else {
            console.warn('Failed to load configuration, hiding upload form');
            const uploadForm = document.getElementById('uploadForm');
            if (uploadForm) {
                uploadForm.style.display = 'none';
            }
        }
    } catch (error) {
        console.error('Error loading configuration:', error);
        const uploadForm = document.getElementById('uploadForm');
        if (uploadForm) {
            uploadForm.style.display = 'none';
        }
    }
}
