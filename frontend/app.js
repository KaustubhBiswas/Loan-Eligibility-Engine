/**
 * Loan Eligibility Engine - Frontend Application
 */

// Configuration
const CONFIG = {
    apiBaseUrl: window.location.origin.includes('localhost') 
        ? 'http://localhost:8080' 
        : window.location.origin,
    maxFileSize: 10 * 1024 * 1024, // 10MB
    allowedFileTypes: ['text/csv', 'application/vnd.ms-excel'],
};

// DOM Elements
const elements = {
    uploadArea: document.getElementById('uploadArea'),
    fileInput: document.getElementById('fileInput'),
    browseBtn: document.getElementById('browseBtn'),
    fileSelected: document.getElementById('fileSelected'),
    fileName: document.getElementById('fileName'),
    fileSize: document.getElementById('fileSize'),
    removeFile: document.getElementById('removeFile'),
    uploadBtn: document.getElementById('uploadBtn'),
    uploadProgress: document.getElementById('uploadProgress'),
    progressPercent: document.getElementById('progressPercent'),
    progressFill: document.getElementById('progressFill'),
    progressStatus: document.getElementById('progressStatus'),
    uploadSuccess: document.getElementById('uploadSuccess'),
    uploadError: document.getElementById('uploadError'),
    errorMessage: document.getElementById('errorMessage'),
    uploadAnother: document.getElementById('uploadAnother'),
    tryAgain: document.getElementById('tryAgain'),
    downloadSample: document.getElementById('downloadSample'),
    totalRows: document.getElementById('totalRows'),
    validRows: document.getElementById('validRows'),
    errorRows: document.getElementById('errorRows'),
    toastContainer: document.getElementById('toastContainer'),
};

// State
let selectedFile = null;

/**
 * Initialize the application
 */
function init() {
    setupEventListeners();
}

/**
 * Setup event listeners
 */
function setupEventListeners() {
    // Drag and drop
    elements.uploadArea.addEventListener('dragover', handleDragOver);
    elements.uploadArea.addEventListener('dragleave', handleDragLeave);
    elements.uploadArea.addEventListener('drop', handleDrop);
    elements.uploadArea.addEventListener('click', () => elements.fileInput.click());

    // File input
    elements.browseBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        elements.fileInput.click();
    });
    elements.fileInput.addEventListener('change', handleFileSelect);

    // File actions
    elements.removeFile.addEventListener('click', removeSelectedFile);
    elements.uploadBtn.addEventListener('click', uploadFile);

    // Reset actions
    elements.uploadAnother.addEventListener('click', resetUpload);
    elements.tryAgain.addEventListener('click', resetUpload);

    // Download sample
    elements.downloadSample.addEventListener('click', downloadSampleCSV);
}

/**
 * Handle drag over event
 */
function handleDragOver(e) {
    e.preventDefault();
    e.stopPropagation();
    elements.uploadArea.classList.add('drag-over');
}

/**
 * Handle drag leave event
 */
function handleDragLeave(e) {
    e.preventDefault();
    e.stopPropagation();
    elements.uploadArea.classList.remove('drag-over');
}

/**
 * Handle file drop
 */
function handleDrop(e) {
    e.preventDefault();
    e.stopPropagation();
    elements.uploadArea.classList.remove('drag-over');

    const files = e.dataTransfer.files;
    if (files.length > 0) {
        validateAndSelectFile(files[0]);
    }
}

/**
 * Handle file selection from input
 */
function handleFileSelect(e) {
    const files = e.target.files;
    if (files.length > 0) {
        validateAndSelectFile(files[0]);
    }
}

/**
 * Validate and select file
 */
function validateAndSelectFile(file) {
    // Validate file type
    if (!file.name.endsWith('.csv')) {
        showToast('Please select a CSV file', 'error');
        return;
    }

    // Validate file size
    if (file.size > CONFIG.maxFileSize) {
        showToast('File size exceeds 10MB limit', 'error');
        return;
    }

    selectedFile = file;
    showSelectedFile(file);
}

/**
 * Show selected file info
 */
function showSelectedFile(file) {
    elements.fileName.textContent = file.name;
    elements.fileSize.textContent = formatFileSize(file.size);
    
    elements.uploadArea.classList.add('hidden');
    elements.fileSelected.classList.remove('hidden');
}

/**
 * Remove selected file
 */
function removeSelectedFile() {
    selectedFile = null;
    elements.fileInput.value = '';
    
    elements.fileSelected.classList.add('hidden');
    elements.uploadArea.classList.remove('hidden');
}

/**
 * Upload file to S3 via presigned URL
 */
async function uploadFile() {
    if (!selectedFile) {
        showToast('Please select a file first', 'error');
        return;
    }

    try {
        showProgress();
        updateProgress(10, 'Preparing upload...');

        // Direct upload to Go server
        const formData = new FormData();
        formData.append('file', selectedFile);

        updateProgress(30, 'Uploading file...');

        const response = await fetch(`${CONFIG.apiBaseUrl}/api/upload`, {
            method: 'POST',
            body: formData
        });

        updateProgress(70, 'Processing file...');

        if (!response.ok) {
            throw new Error('Upload failed');
        }

        const result = await response.json();
        
        if (!result.success) {
            throw new Error(result.error || 'Upload failed');
        }

        updateProgress(100, 'Complete!');

        // Show success
        setTimeout(() => {
            showSuccess({
                total: result.data.total_rows || 0,
                valid: result.data.valid_users || 0,
                errors: result.data.errors || 0,
                matches: result.data.matches_found || 0
            });
        }, 500);

    } catch (error) {
        console.error('Upload failed:', error);
        showError(error.message || 'Upload failed. Please try again.');
    }
}

/**
 * Get presigned URL from API
 */
async function getPresignedUrl(filename) {
    const response = await fetch(`${CONFIG.apiBaseUrl}/api/presigned-url`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            filename: filename,
            content_type: 'text/csv',
        }),
    });

    if (!response.ok) {
        throw new Error('Failed to get upload URL');
    }

    return response.json();
}

/**
 * Upload file to S3 using presigned URL
 */
async function uploadToS3(presignedUrl, file) {
    const response = await fetch(presignedUrl, {
        method: 'PUT',
        headers: {
            'Content-Type': 'text/csv',
        },
        body: file,
    });

    if (!response.ok) {
        throw new Error('Failed to upload file to S3');
    }
}

/**
 * Wait for file processing to complete
 */
async function waitForProcessing(fileKey) {
    // In production, this would poll an API endpoint or use WebSocket
    // For demo, we'll simulate a delay
    return new Promise(resolve => setTimeout(resolve, 2000));
}

/**
 * Show progress UI
 */
function showProgress() {
    elements.fileSelected.classList.add('hidden');
    elements.uploadProgress.classList.remove('hidden');
    elements.uploadSuccess.classList.add('hidden');
    elements.uploadError.classList.add('hidden');
}

/**
 * Update progress bar
 */
function updateProgress(percent, status) {
    elements.progressPercent.textContent = `${percent}%`;
    elements.progressFill.style.width = `${percent}%`;
    elements.progressStatus.textContent = status;
}

/**
 * Show success UI
 */
function showSuccess(stats) {
    elements.uploadProgress.classList.add('hidden');
    elements.uploadSuccess.classList.remove('hidden');

    elements.totalRows.textContent = stats.total;
    elements.validRows.textContent = stats.valid;
    elements.errorRows.textContent = stats.errors;

    showToast('File uploaded successfully!', 'success');
}

/**
 * Show error UI
 */
function showError(message) {
    elements.uploadProgress.classList.add('hidden');
    elements.uploadError.classList.remove('hidden');
    elements.errorMessage.textContent = message;

    showToast(message, 'error');
}

/**
 * Reset upload UI
 */
function resetUpload() {
    selectedFile = null;
    elements.fileInput.value = '';

    elements.uploadArea.classList.remove('hidden');
    elements.fileSelected.classList.add('hidden');
    elements.uploadProgress.classList.add('hidden');
    elements.uploadSuccess.classList.add('hidden');
    elements.uploadError.classList.add('hidden');

    updateProgress(0, '');
}

/**
 * Download sample CSV file
 */
function downloadSampleCSV() {
    const csvContent = `name,email,age,annual_income,credit_score,employment_status,loan_amount_required,location
Rahul Sharma,rahul.sharma@email.com,32,850000,750,salaried,500000,Mumbai
Priya Patel,priya.patel@email.com,28,600000,680,self_employed,300000,Delhi
Amit Kumar,amit.kumar@email.com,45,1200000,800,business,1000000,Bangalore
Sneha Reddy,sneha.reddy@email.com,35,480000,650,salaried,200000,Hyderabad
Vikram Singh,vikram.singh@email.com,29,720000,720,salaried,400000,Chennai`;

    const blob = new Blob([csvContent], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    
    const link = document.createElement('a');
    link.href = url;
    link.download = 'sample_users.csv';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    
    URL.revokeObjectURL(url);
    showToast('Sample CSV downloaded', 'success');
}

/**
 * Show toast notification
 */
function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.innerHTML = `
        <span>${message}</span>
    `;

    elements.toastContainer.appendChild(toast);

    // Auto remove after 3 seconds
    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transform = 'translateX(100px)';
        setTimeout(() => toast.remove(), 300);
    }, 3000);
}

/**
 * Format file size
 */
function formatFileSize(bytes) {
    if (bytes === 0) return '0 Bytes';
    
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', init);
