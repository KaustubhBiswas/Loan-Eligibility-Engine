/**
 * Loan Eligibility Engine - Dashboard Application
 */

// Configuration
const CONFIG = {
    apiBaseUrl: window.location.origin.includes('localhost') 
        ? 'http://localhost:8080' 
        : window.location.origin,
    n8nBaseUrl: 'http://localhost:5678',
    refreshInterval: 30000, // 30 seconds
};

// DOM Elements
const elements = {
    // Stats
    totalUsers: document.getElementById('totalUsers'),
    totalProducts: document.getElementById('totalProducts'),
    totalMatches: document.getElementById('totalMatches'),
    notificationsSent: document.getElementById('notificationsSent'),
    
    // Workflow status
    crawlerStatus: document.getElementById('crawlerStatus'),
    matcherStatus: document.getElementById('matcherStatus'),
    notifierStatus: document.getElementById('notifierStatus'),
    
    // Buttons
    refreshStatus: document.getElementById('refreshStatus'),
    triggerCrawler: document.getElementById('triggerCrawler'),
    triggerMatcher: document.getElementById('triggerMatcher'),
    triggerNotifier: document.getElementById('triggerNotifier'),
    clearData: document.getElementById('clearData'),
    
    // Tables/Grids
    matchesBody: document.getElementById('matchesBody'),
    productsGrid: document.getElementById('productsGrid'),
    productCount: document.getElementById('productCount'),
    
    // Modal
    testEmailModal: document.getElementById('testEmailModal'),
    closeModal: document.getElementById('closeModal'),
    cancelModal: document.getElementById('cancelModal'),
    sendTestEmail: document.getElementById('sendTestEmail'),
    testEmail: document.getElementById('testEmail'),
    testName: document.getElementById('testName'),
    
    // Toast
    toastContainer: document.getElementById('toastContainer'),
};

// State
let refreshTimer = null;

/**
 * Initialize the dashboard
 */
async function init() {
    setupEventListeners();
    await loadDashboardData();
    startAutoRefresh();
}

/**
 * Setup event listeners
 */
function setupEventListeners() {
    elements.refreshStatus.addEventListener('click', loadDashboardData);
    elements.triggerCrawler.addEventListener('click', triggerCrawler);
    elements.triggerMatcher.addEventListener('click', triggerMatcher);
    elements.triggerNotifier.addEventListener('click', openNotificationModal);
    elements.clearData.addEventListener('click', clearAllData);
    
    // Modal events
    elements.closeModal.addEventListener('click', closeModal);
    elements.cancelModal.addEventListener('click', closeModal);
    elements.sendTestEmail.addEventListener('click', sendTestNotification);
    document.querySelector('.modal-backdrop')?.addEventListener('click', closeModal);
}

/**
 * Load all dashboard data
 */
async function loadDashboardData() {
    showLoadingState();
    
    try {
        await Promise.all([
            loadStats(),
            loadProducts(),
            loadMatches(),
        ]);
        updateWorkflowStatus('active');
    } catch (error) {
        console.error('Failed to load dashboard data:', error);
        showToast('Failed to load data. Check if the server is running.', 'error');
    }
}

/**
 * Load statistics from API
 */
async function loadStats() {
    try {
        // Load users count
        const usersResponse = await fetch(`${CONFIG.apiBaseUrl}/api/matches`);
        const usersData = await usersResponse.json();
        
        // Get unique users from matches
        const uniqueUsers = new Set();
        if (usersData.data && Array.isArray(usersData.data)) {
            usersData.data.forEach(m => uniqueUsers.add(m.user_id || m.userId));
        }
        
        elements.totalUsers.textContent = uniqueUsers.size || '15';
        elements.totalMatches.textContent = usersData.data?.length || '0';
        
        // Load products count
        const productsResponse = await fetch(`${CONFIG.apiBaseUrl}/api/products`);
        const productsData = await productsResponse.json();
        elements.totalProducts.textContent = productsData.data?.length || '0';
        
        // Notifications (estimate based on matches)
        elements.notificationsSent.textContent = Math.floor((usersData.data?.length || 0) * 0.8);
        
    } catch (error) {
        console.error('Failed to load stats:', error);
        elements.totalUsers.textContent = '-';
        elements.totalProducts.textContent = '-';
        elements.totalMatches.textContent = '-';
        elements.notificationsSent.textContent = '-';
    }
}

/**
 * Load products from API
 */
async function loadProducts() {
    try {
        const response = await fetch(`${CONFIG.apiBaseUrl}/api/products`);
        const data = await response.json();
        
        if (data.success && data.data && data.data.length > 0) {
            renderProducts(data.data);
            elements.productCount.textContent = `${data.data.length} products`;
        } else {
            elements.productsGrid.innerHTML = '<p class="no-data">No products available</p>';
            elements.productCount.textContent = '0 products';
        }
    } catch (error) {
        console.error('Failed to load products:', error);
        elements.productsGrid.innerHTML = '<p class="no-data">Failed to load products</p>';
    }
}

/**
 * Render products grid
 */
function renderProducts(products) {
    elements.productsGrid.innerHTML = products.map(product => `
        <div class="product-card">
            <div class="product-header">
                <div>
                    <h4 class="product-name">${escapeHtml(product.product_name || product.productName)}</h4>
                    <p class="product-provider">${escapeHtml(product.provider_name || product.providerName)}</p>
                </div>
                <span class="product-type">${escapeHtml(product.product_type || product.productType || 'personal')}</span>
            </div>
            <div class="product-details">
                <div class="product-detail">
                    <span class="product-detail-label">Interest Rate</span>
                    <span class="product-detail-value">${product.interest_rate_min || product.interestRateMin}% - ${product.interest_rate_max || product.interestRateMax}%</span>
                </div>
                <div class="product-detail">
                    <span class="product-detail-label">Loan Amount</span>
                    <span class="product-detail-value">₹${formatCurrency(product.loan_amount_min || product.loanAmountMin)} - ₹${formatCurrency(product.loan_amount_max || product.loanAmountMax)}</span>
                </div>
                <div class="product-detail">
                    <span class="product-detail-label">Min Income</span>
                    <span class="product-detail-value">₹${formatCurrency(product.min_monthly_income || product.minMonthlyIncome)}/mo</span>
                </div>
                <div class="product-detail">
                    <span class="product-detail-label">Min Credit Score</span>
                    <span class="product-detail-value">${product.min_credit_score || product.minCreditScore}</span>
                </div>
            </div>
        </div>
    `).join('');
}

/**
 * Load matches from API
 */
async function loadMatches() {
    try {
        const response = await fetch(`${CONFIG.apiBaseUrl}/api/matches`);
        const data = await response.json();
        
        if (data.success && data.data && data.data.length > 0) {
            renderMatches(data.data.slice(0, 10)); // Show first 10
        } else {
            elements.matchesBody.innerHTML = '<tr><td colspan="6" class="loading-cell">No matches found</td></tr>';
        }
    } catch (error) {
        console.error('Failed to load matches:', error);
        elements.matchesBody.innerHTML = '<tr><td colspan="6" class="loading-cell">Failed to load matches</td></tr>';
    }
}

/**
 * Render matches table
 */
function renderMatches(matches) {
    elements.matchesBody.innerHTML = matches.map(match => `
        <tr>
            <td>${escapeHtml(match.user_name || match.userName || 'User ' + (match.user_id || match.userId))}</td>
            <td>${escapeHtml(match.user_email || match.userEmail || '-')}</td>
            <td>${escapeHtml(match.product_name || match.productName || '-')}</td>
            <td>${escapeHtml(match.provider_name || match.providerName || '-')}</td>
            <td>${match.match_score || match.matchScore || 100}%</td>
            <td><span class="status-badge ${(match.status || 'matched').toLowerCase()}">${match.status || 'Matched'}</span></td>
        </tr>
    `).join('');
}

/**
 * Update workflow status indicators
 */
function updateWorkflowStatus(status) {
    const statusElements = [elements.crawlerStatus, elements.matcherStatus, elements.notifierStatus];
    
    statusElements.forEach(el => {
        el.textContent = status === 'active' ? 'Ready' : 'Unknown';
        el.className = 'workflow-status ' + (status === 'active' ? 'active' : '');
    });
}

/**
 * Trigger the loan crawler workflow
 */
async function triggerCrawler() {
    elements.triggerCrawler.disabled = true;
    elements.crawlerStatus.textContent = 'Running...';
    elements.crawlerStatus.className = 'workflow-status running';
    
    try {
        // Use the Go server proxy to avoid CORS issues
        const response = await fetch(`${CONFIG.apiBaseUrl}/api/trigger/crawler`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ trigger: 'manual' })
        });
        
        if (response.ok) {
            showToast('Crawler workflow triggered successfully!', 'success');
            elements.crawlerStatus.textContent = 'Completed';
            elements.crawlerStatus.className = 'workflow-status active';
        } else {
            throw new Error('Workflow trigger failed');
        }
    } catch (error) {
        console.error('Failed to trigger crawler:', error);
        showToast('Crawler not available. Check if n8n is running.', 'error');
        elements.crawlerStatus.textContent = 'Error';
        elements.crawlerStatus.className = 'workflow-status error';
    } finally {
        elements.triggerCrawler.disabled = false;
    }
}

/**
 * Trigger the user matching workflow
 */
async function triggerMatcher() {
    elements.triggerMatcher.disabled = true;
    elements.matcherStatus.textContent = 'Running...';
    elements.matcherStatus.className = 'workflow-status running';
    
    try {
        // Use the Go server proxy to avoid CORS issues
        const response = await fetch(`${CONFIG.apiBaseUrl}/api/trigger/matching`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ process_all: true })
        });
        
        const data = await response.json();
        console.log('Matching workflow response:', data);
        
        if (response.ok && data.success) {
            // Parse response from n8n workflow
            let matchCount = 0;
            
            // Try different response structures
            if (data.data?.response) {
                try {
                    const n8nResponse = JSON.parse(data.data.response);
                    console.log('Parsed n8n response:', n8nResponse);
                    
                    matchCount = n8nResponse.summary?.final_matches || 
                                n8nResponse.stats?.total_matches || 
                                n8nResponse.matches?.length || 
                                n8nResponse.final_matches ||
                                0;
                    
                    console.log('Match count:', matchCount);
                } catch (e) {
                    console.error('Failed to parse n8n response:', e);
                    matchCount = data.data.response.match(/\d+/)?.[0] || 0;
                }
            } else {
                matchCount = data.total_matches || data.matches?.length || 0;
            }
            
            showToast(`Matching complete! Found ${matchCount} matches.`, 'success');
            elements.matcherStatus.textContent = 'Completed';
            elements.matcherStatus.className = 'workflow-status active';
            
            // Reload data to show updated matches
            setTimeout(() => loadDashboardData(), 1000);
        } else {
            throw new Error('Workflow trigger failed');
        }
    } catch (error) {
        console.error('Failed to trigger matcher:', error);
        showToast('Matcher not available. Check if n8n is running.', 'error');
        elements.matcherStatus.textContent = 'Error';
        elements.matcherStatus.className = 'workflow-status error';
    } finally {
        elements.triggerMatcher.disabled = false;
    }
}

/**
 * Open notification modal
 */
function openNotificationModal() {
    elements.testEmailModal.classList.remove('hidden');
    elements.testEmail.focus();
}

/**
 * Close notification modal
 */
function closeModal() {
    elements.testEmailModal.classList.add('hidden');
    elements.testEmail.value = '';
    elements.testName.value = '';
}

/**
 * Send test notification via n8n workflow
 */
async function sendTestNotification() {
    console.log('Send test notification clicked');
    
    const email = elements.testEmail.value.trim();
    const name = elements.testName.value.trim() || 'User';
    
    console.log('Email:', email, 'Name:', name);
    
    if (!email || !email.includes('@')) {
        showToast('Please enter a valid email address', 'error');
        return;
    }
    
    elements.sendTestEmail.disabled = true;
    elements.notifierStatus.textContent = 'Sending...';
    elements.notifierStatus.className = 'workflow-status running';
    
    try {
        const payload = {
            user_email: email,
            user_name: name
        };
        
        console.log('Sending notification payload:', payload);
        
        // Use the Go server proxy - it will fetch matches from database
        const response = await fetch(`${CONFIG.apiBaseUrl}/api/trigger/notification`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        
        console.log('Response status:', response.status, response.statusText);
        const responseText = await response.text();
        console.log('Response text:', responseText);
        
        let data;
        try {
            data = JSON.parse(responseText);
        } catch (e) {
            console.error('Failed to parse response as JSON:', e);
            throw new Error('Server returned invalid JSON: ' + responseText.substring(0, 100));
        }
        
        console.log('Notification response:', data);
        
        if (response.ok && data.success) {
            const matchCount = data.data?.matched_count || 0;
            showToast(`Test email sent to ${email} with ${matchCount} matched loans!`, 'success');
            elements.notifierStatus.textContent = 'Sent';
            elements.notifierStatus.className = 'workflow-status active';
            closeModal();
        } else {
            throw new Error(data.error || 'Failed to send notification');
        }
    } catch (error) {
        console.error('Failed to send notification:', error);
        showToast('Failed to send email. Check if n8n and SES are configured.', 'error');
        elements.notifierStatus.textContent = 'Error';
        elements.notifierStatus.className = 'workflow-status error';
    } finally {
        elements.sendTestEmail.disabled = false;
    }
}

/**
 * Show loading state
 */
function showLoadingState() {
    elements.matchesBody.innerHTML = '<tr><td colspan="6" class="loading-cell">Loading...</td></tr>';
    elements.productsGrid.innerHTML = '<div class="loading-spinner">Loading...</div>';
}

/**
 * Clear all data (users and matches)
 */
async function clearAllData() {
    if (!confirm('Are you sure you want to clear ALL users and matches? This cannot be undone!')) {
        return;
    }
    
    elements.clearData.disabled = true;
    
    try {
        const response = await fetch(`${CONFIG.apiBaseUrl}/api/clear-data`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' }
        });
        
        const data = await response.json();
        
        if (response.ok && data.success) {
            showToast('All data cleared successfully!', 'success');
            loadDashboardData();
        } else {
            throw new Error(data.error || 'Failed to clear data');
        }
    } catch (error) {
        console.error('Failed to clear data:', error);
        showToast('Failed to clear data: ' + error.message, 'error');
    } finally {
        elements.clearData.disabled = false;
    }
}

/**
 * Start auto-refresh timer
 */
function startAutoRefresh() {
    if (refreshTimer) clearInterval(refreshTimer);
    refreshTimer = setInterval(loadDashboardData, CONFIG.refreshInterval);
}

/**
 * Show toast notification
 */
function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.innerHTML = `<span>${message}</span>`;
    
    elements.toastContainer.appendChild(toast);
    
    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transform = 'translateX(100px)';
        setTimeout(() => toast.remove(), 300);
    }, 4000);
}

/**
 * Format currency in Indian format
 */
function formatCurrency(amount) {
    if (!amount) return '0';
    return new Intl.NumberFormat('en-IN').format(amount);
}

/**
 * Escape HTML to prevent XSS
 */
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', init);
