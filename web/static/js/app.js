// LLM API Frontend JavaScript

const API_BASE = '';

// Utility functions
async function apiRequest(url, options = {}) {
    const token = localStorage.getItem('token');
    const headers = {
        'Content-Type': 'application/json',
        ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
        ...options.headers
    };

    try {
        const response = await fetch(url, {
            ...options,
            headers
        });

        const data = await response.json();

        if (!response.ok) {
            throw new Error(data.error || 'Request failed');
        }

        return data;
    } catch (error) {
        console.error('API request failed:', error);
        throw error;
    }
}

// Auth functions
async function login(username, password) {
    const data = await apiRequest('/web/auth/login', {
        method: 'POST',
        body: JSON.stringify({ username, password })
    });

    if (data.user) {
        localStorage.setItem('user', JSON.stringify(data.user));
        if (data.token) {
            localStorage.setItem('token', data.token);
        }
    }

    return data;
}

function logout() {
    localStorage.removeItem('user');
    localStorage.removeItem('token');
    window.location.href = '/login.html';
}

function getCurrentUser() {
    const userStr = localStorage.getItem('user');
    return userStr ? JSON.parse(userStr) : null;
}

function isAdmin() {
    const user = getCurrentUser();
    return user && user.is_admin;
}

// API Key functions
async function getMyAPIKeys() {
    return apiRequest('/web/user/apikeys');
}

async function createAPIKey(keyName) {
    return apiRequest('/web/user/apikeys', {
        method: 'POST',
        body: JSON.stringify({ key_name: keyName })
    });
}

async function deleteAPIKey(id) {
    return apiRequest(`/web/user/apikeys/${id}`, {
        method: 'DELETE'
    });
}

// Usage functions
async function getMyUsage(page = 1, pageSize = 10) {
    return apiRequest(`/web/user/usage?page=${page}&page_size=${pageSize}`);
}

// Admin functions
async function getUsers(page = 1, pageSize = 10) {
    return apiRequest(`/admin/users?page=${page}&page_size=${pageSize}`);
}

async function createUser(username, password, requestLimit) {
    return apiRequest('/admin/users', {
        method: 'POST',
        body: JSON.stringify({
            username,
            password,
            request_limit: requestLimit
        })
    });
}

async function updateUser(id, requestLimit) {
    return apiRequest(`/admin/users/${id}`, {
        method: 'PUT',
        body: JSON.stringify({ request_limit: requestLimit })
    });
}

async function deleteUser(id) {
    return apiRequest(`/admin/users/${id}`, {
        method: 'DELETE'
    });
}

async function getAPIKeys(page = 1, pageSize = 10) {
    return apiRequest(`/admin/apikeys?page=${page}&page_size=${pageSize}`);
}

async function createAdminAPIKey(userId, keyName) {
    return apiRequest('/admin/apikeys', {
        method: 'POST',
        body: JSON.stringify({
            user_id: userId,
            key_name: keyName
        })
    });
}

async function resetAPIKey(id) {
    return apiRequest(`/admin/apikeys/${id}/reset`, {
        method: 'POST'
    });
}

async function toggleAPIKey(id) {
    return apiRequest(`/admin/apikeys/${id}/toggle`, {
        method: 'POST'
    });
}

async function deleteAPIKeyAdmin(id) {
    return apiRequest(`/admin/apikeys/${id}`, {
        method: 'DELETE'
    });
}

async function getUsage(page = 1, pageSize = 10) {
    return apiRequest(`/admin/usage?page=${page}&page_size=${pageSize}`);
}

async function getUserUsage(userId, page = 1, pageSize = 10) {
    return apiRequest(`/admin/users/${userId}/usage?page=${page}&page_size=${pageSize}`);
}

async function getStats() {
    return apiRequest('/admin/stats');
}

// Export functions
if (typeof module !== 'undefined' && module.exports) {
    module.exports = {
        apiRequest,
        login,
        logout,
        getCurrentUser,
        isAdmin,
        getMyAPIKeys,
        createAPIKey,
        deleteAPIKey,
        getMyUsage,
        getUsers,
        createUser,
        updateUser,
        deleteUser,
        getAPIKeys,
        createAdminAPIKey,
        resetAPIKey,
        toggleAPIKey,
        deleteAPIKeyAdmin,
        getUsage,
        getUserUsage,
        getStats
    };
}
