// admin.js - Admin page JavaScript

// Copy API Key
function copyKey(keyValue) {
    navigator.clipboard.writeText(keyValue).then(() => {
        alert('已复制到剪贴板');
    }).catch(err => {
        console.error('复制失败:', err);
        const textarea = document.createElement('textarea');
        textarea.value = keyValue;
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
        alert('已复制到剪贴板');
    });
}

// Get auth header
function getAuthHeader() {
    const token = localStorage.getItem('token');
    return token ? { 'Authorization': 'Bearer ' + token } : {};
}

// Check response for token expiration
async function checkResponse(response) {
    if (response.status === 401) {
        const data = await response.json();
        if (data.error && (data.error.includes('token') || data.error.includes('Token'))) {
            localStorage.removeItem('user');
            localStorage.removeItem('token');
            window.location.href = '/login.html';
            throw new Error('Token expired');
        }
    }
    return response;
}

// Unified fetch wrapper
async function adminFetch(url, options = {}) {
    const response = await fetch(url, { ...options, headers: { ...getAuthHeader(), ...options.headers } });
    await checkResponse(response);
    return response;
}

// Check if admin is logged in
const user = JSON.parse(localStorage.getItem('user'));
if (!user || !user.is_admin) {
    window.location.href = '/login.html';
}

document.getElementById('adminUsername').textContent = user.username;

// Pagination variables
let usersPage = 1;
let usersPageSize = 10;
let usersTotal = 0;
let usersSearch = '';
let activationPage = 1;
let activationPageSize = 10;
let activationTotal = 0;
let apiKeysPage = 1;
let apiKeysPageSize = 10;
let apiKeysTotal = 0;
let apiKeysKeySearch = '';
let apiKeysUserIdSearch = '';

// Tab switching
document.querySelectorAll('.tab').forEach(tab => {
    tab.addEventListener('click', () => {
        document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
        document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
        tab.classList.add('active');
        document.getElementById(tab.dataset.tab + '-tab').classList.add('active');

        if (tab.dataset.tab === 'users') loadUsers();
        if (tab.dataset.tab === 'activation') loadActivationUsers();
        if (tab.dataset.tab === 'apikeys') loadAPIKeys();
        if (tab.dataset.tab === 'usage') {
            loadUsage();
            loadUpstreamUsage();
        }
    });
});

// Load Users
async function loadUsers() {
    try {
        let url = '/admin/users?page=' + usersPage + '&page_size=' + usersPageSize;
        if (usersSearch) {
            url += '&username=' + encodeURIComponent(usersSearch);
        }
        if (usersLevelFilter) {
            url += '&level=' + usersLevelFilter;
        }
        if (usersIdFilter) {
            url += '&id=' + encodeURIComponent(usersIdFilter);
        }
        if (usersGmlFilter) {
            url += '&use_gml=' + usersGmlFilter;
        }
        if (usersKimiFilter) {
            url += '&use_kimi=' + usersKimiFilter;
        }
        var sort = document.getElementById('sortSelect').value;
        url += '&sort=' + sort;
        const response = await adminFetch(url);
        const data = await response.json();

        usersTotal = data.total || 0;

        const container = document.getElementById('usersList');
        if (!data.data || data.data.length === 0) {
            container.innerHTML = '<p class="empty">暂无用户</p>';
            return;
        }

        var html = '<div class="table-wrapper"><table><thead><tr><th>ID</th><th>用户名</th><th>额度限制</th><th>已用次数</th><th>等级</th><th>Gml</th><th>Kimi</th><th>周额度限制</th><th>过期时间</th><th>状态</th><th>备注</th><th>创建时间</th><th>操作</th></tr></thead><tbody>';
        data.data.forEach(function(u) {
            var isExpired = u.expires_at && new Date(u.expires_at) < new Date();
            var status = u.expires_at ? (isExpired ? '<span class="status inactive">已过期</span>' : '<span class="status active">正常</span>') : '<span class="status active">永不过期</span>';
            var levelText = u.level === 2 ? '<span class="status active">高速</span>' : '<span class="status">普通</span>';
            var weeklyLimitText = u.has_weekly_limit === -1 ? '<span class="status active">无限制</span>' : (u.has_weekly_limit === 1 ? '有限制' : '-');
            var gmlText = u.use_gml === 1 ? '<span class="status active">是</span>' : '<span class="status">否</span>';
            var kimiText = u.use_kimi === 1 ? '<span class="status active">是</span>' : '<span class="status">否</span>';
            html += '<tr><td>' + u.id + '</td><td>' + u.username + '</td><td>' + u.request_limit + '</td><td>' + u.request_count + '</td><td>' + levelText + '</td><td>' + gmlText + '</td><td>' + kimiText + '</td><td>' + weeklyLimitText + '</td><td>' + (u.expires_at || '-') + '</td><td>' + status + '</td><td>' + (u.remark || '-') + '</td><td>' + u.created_at + '</td><td><button class="btn btn-sm" onclick="editUser(' + u.id + ', ' + u.request_limit + ', \'' + (u.expires_at || '') + '\', \'' + u.username + '\', \'' + (u.remark || '') + '\', ' + (u.level || 1) + ', ' + (u.has_weekly_limit !== undefined ? u.has_weekly_limit : -1) + ', ' + (u.use_gml !== undefined ? u.use_gml : -1) + ', ' + (u.use_kimi !== undefined ? u.use_kimi : -1) + ')">编辑</button> <button class="btn btn-sm btn-danger" onclick="deleteUser(' + u.id + ')">删除</button></td></tr>';
        });
        html += '</tbody></table></div>';

        var prevDisabled = usersPage <= 1 ? 'disabled' : '';
        var nextDisabled = usersPage >= Math.ceil(usersTotal / usersPageSize) ? 'disabled' : '';
        var totalPages = Math.ceil(usersTotal / usersPageSize) || 1;

        html += '<div class="pagination"><button class="btn btn-sm" onclick="changeUsersPage(1)">首页</button> <button class="btn btn-sm" ' + prevDisabled + ' onclick="changeUsersPage(' + (usersPage - 1) + ')">上一页</button> <span>第 ' + usersPage + ' / ' + totalPages + ' 页</span> <button class="btn btn-sm" ' + nextDisabled + ' onclick="changeUsersPage(' + (usersPage + 1) + ')">下一页</button> <button class="btn btn-sm" onclick="changeUsersPage(' + totalPages + ')">尾页</button></div>';

        container.innerHTML = html;
    } catch (error) {
        console.error('Failed to load users:', error);
    }
}

function changeUsersPage(newPage) {
    var totalPages = Math.ceil(usersTotal / usersPageSize);
    if (newPage < 1 || newPage > totalPages) return;
    usersPage = newPage;
    loadUsers();
}

let usersLevelFilter = '';
let usersIdFilter = '';
let usersGmlFilter = '';
let usersKimiFilter = '';

function searchUsers() {
    var searchInput = document.getElementById('userSearchInput');
    usersSearch = searchInput.value.trim();

    var levelFilter = document.getElementById('levelFilter');
    usersLevelFilter = levelFilter.value;

    var userIdFilter = document.getElementById('userIdFilter');
    usersIdFilter = userIdFilter.value.trim();

    var gmlFilter = document.getElementById('gmlFilter');
    usersGmlFilter = gmlFilter.value;

    var kimiFilter = document.getElementById('kimiFilter');
    usersKimiFilter = kimiFilter.value;

    usersPage = 1;
    loadUsers();
}

function changeActivationPage(newPage) {
    var totalPages = Math.ceil(activationTotal / activationPageSize);
    if (newPage < 1 || newPage > totalPages) return;
    activationPage = newPage;
    loadActivationUsers();
}

// Load Activation Users
async function loadActivationUsers() {
    try {
        const response = await adminFetch('/admin/activation-users?page=' + activationPage + '&page_size=' + activationPageSize);
        const data = await response.json();

        activationTotal = data.total || 0;

        const container = document.getElementById('activationUsersList');
        if (!data.data || data.data.length === 0) {
            container.innerHTML = '<p class="empty">暂无待激活用户</p>';
            return;
        }

        var html = '<div class="table-wrapper"><table><thead><tr><th>ID</th><th>用户名</th><th>有效天数</th><th>请求限制</th><th>等级</th><th>Gml</th><th>Kimi</th><th>周额度限制</th><th>备注</th><th>创建时间</th><th>操作</th></tr></thead><tbody>';
        data.data.forEach(function(u) {
            var levelText = u.level === 2 ? '<span class="status active">高速</span>' : '<span class="status">普通</span>';
            var weeklyLimitText = u.has_weekly_limit === -1 ? '无限制' : (u.has_weekly_limit === 1 ? '有限制' : '-');
            var gmlText = u.use_gml === 1 ? '<span class="status active">是</span>' : '<span class="status">否</span>';
            var kimiText = u.use_kimi === 1 ? '<span class="status active">是</span>' : '<span class="status">否</span>';
            html += '<tr><td>' + u.id + '</td><td>' + u.username + '</td><td>' + u.valid_days + ' 天</td><td>' + u.request_limit + '</td><td>' + levelText + '</td><td>' + gmlText + '</td><td>' + kimiText + '</td><td>' + weeklyLimitText + '</td><td>' + (u.remarks || '-') + '</td><td>' + u.created_at + '</td><td><button class="btn btn-sm btn-danger" onclick="deleteActivationUser(' + u.id + ')">删除</button></td></tr>';
        });
        html += '</tbody></table></div>';

        var prevDisabled = activationPage <= 1 ? 'disabled' : '';
        var nextDisabled = activationPage >= Math.ceil(activationTotal / activationPageSize) ? 'disabled' : '';
        var totalPages = Math.ceil(activationTotal / activationPageSize) || 1;

        html += '<div class="pagination"><button class="btn btn-sm" ' + prevDisabled + ' onclick="changeActivationPage(1)">首页</button> <button class="btn btn-sm" ' + prevDisabled + ' onclick="changeActivationPage(' + (activationPage - 1) + ')">上一页</button> <span>第 ' + activationPage + ' / ' + totalPages + ' 页</span> <button class="btn btn-sm" ' + nextDisabled + ' onclick="changeActivationPage(' + (activationPage + 1) + ')">下一页</button> <button class="btn btn-sm" ' + nextDisabled + ' onclick="changeActivationPage(' + totalPages + ')">尾页</button></div>';

        container.innerHTML = html;
    } catch (error) {
        console.error('Failed to load activation users:', error);
    }
}

async function deleteActivationUser(id) {
    if (!confirm('确定要删除这个待激活用户吗？')) return;

    try {
        const response = await adminFetch('/admin/activation-users/' + id, { method: 'DELETE' });
        if (response.ok) {
            loadActivationUsers();
        } else {
            alert('删除失败');
        }
    } catch (error) {
        alert('网络错误');
    }
}

// Load API Keys
async function loadAPIKeys() {
    try {
        let url = '/admin/apikeys?page=' + apiKeysPage + '&page_size=' + apiKeysPageSize;
        if (apiKeysKeySearch) {
            url += '&key=' + encodeURIComponent(apiKeysKeySearch);
        }
        if (apiKeysUserIdSearch) {
            url += '&user_id=' + encodeURIComponent(apiKeysUserIdSearch);
        }
        const response = await adminFetch(url);
        const data = await response.json();

        apiKeysTotal = data.total || 0;

        const container = document.getElementById('apiKeysList');
        if (!data.data || data.data.length === 0) {
            container.innerHTML = '<p class="empty">暂无 API Keys</p>';
            return;
        }

        var html = '<div class="table-wrapper"><table><thead><tr><th>ID</th><th>用户</th><th>Key 名称</th><th>Key 值</th><th>状态</th><th>创建时间</th><th>操作</th></tr></thead><tbody>';
        data.data.forEach(function(k) {
            var statusClass = k.is_active ? 'active' : 'inactive';
            var statusText = k.is_active ? '激活' : '禁用';
            var toggleText = k.is_active ? '禁用' : '启用';
            html += '<tr><td>' + k.id + '</td><td>' + (k.username || k.user_id) + '</td><td>' + (k.key_name || '-') + '</td><td><code id="key-' + k.id + '">' + k.key_value + '</code> <button class="btn btn-sm btn-copy" onclick="copyKey(\'' + k.key_value + '\')">复制</button></td><td><span class="status ' + statusClass + '">' + statusText + '</span></td><td>' + k.created_at + '</td><td><button class="btn btn-sm" onclick="resetApiKey(' + k.id + ')">重置</button> <button class="btn btn-sm" onclick="toggleApiKey(' + k.id + ')">' + toggleText + '</button> <button class="btn btn-sm btn-danger" onclick="deleteApiKey(' + k.id + ')">删除</button></td></tr>';
        });
        html += '</tbody></table></div>';

        var prevDisabled = apiKeysPage <= 1 ? 'disabled' : '';
        var nextDisabled = apiKeysPage >= Math.ceil(apiKeysTotal / apiKeysPageSize) ? 'disabled' : '';
        var totalPages = Math.ceil(apiKeysTotal / apiKeysPageSize) || 1;

        html += '<div class="pagination"><button class="btn btn-sm" ' + prevDisabled + ' onclick="changeAPIKeysPage(1)">首页</button> <button class="btn btn-sm" ' + prevDisabled + ' onclick="changeAPIKeysPage(' + (apiKeysPage - 1) + ')">上一页</button> <span>第 ' + apiKeysPage + ' / ' + totalPages + ' 页</span> <button class="btn btn-sm" ' + nextDisabled + ' onclick="changeAPIKeysPage(' + (apiKeysPage + 1) + ')">下一页</button> <button class="btn btn-sm" ' + nextDisabled + ' onclick="changeAPIKeysPage(' + totalPages + ')">末页</button></div>';

        container.innerHTML = html;
    } catch (error) {
        console.error('Failed to load API keys:', error);
    }
}

function changeAPIKeysPage(newPage) {
    var totalPages = Math.ceil(apiKeysTotal / apiKeysPageSize);
    if (newPage < 1 || newPage > totalPages) return;
    apiKeysPage = newPage;
    loadAPIKeys();
}

function searchAPIKeys() {
    var keySearchInput = document.getElementById('apiKeySearchInput');
    var userIdInput = document.getElementById('apiKeyUserIdInput');
    apiKeysKeySearch = keySearchInput.value.trim();
    apiKeysUserIdSearch = userIdInput.value.trim();
    apiKeysPage = 1;
    loadAPIKeys();
}

// Load Usage
async function loadUsage() {
    try {
        const [statsRes, usageRes] = await Promise.all([
            adminFetch('/admin/stats'),
            adminFetch('/admin/usage')
        ]);

        const stats = await statsRes.json();
        const usageData = await usageRes.json();

        document.getElementById('totalRequests').textContent = stats.total_requests || 0;
        document.getElementById('totalTokens').textContent = stats.total_tokens || 0;
        document.getElementById('totalCost').textContent = '$ ' + (stats.total_cost || 0).toFixed(6);
        document.getElementById('totalUsers').textContent = stats.total_users || 0;

        const container = document.getElementById('usageList');
        if (!usageData.data || usageData.data.length === 0) {
            container.innerHTML = '<p class="empty">暂无使用记录</p>';
            return;
        }

        var html = '<div class="table-wrapper"><table><thead><tr><th>ID</th><th>用户ID</th><th>API Key ID</th><th>模型</th><th>Token</th><th>费用</th><th>延迟(ms)</th><th>时间</th></tr></thead><tbody>';
        usageData.data.forEach(function(log) {
            html += '<tr><td>' + log.id + '</td><td>' + log.user_id + '</td><td>' + log.api_key_id + '</td><td>' + log.model + '</td><td>' + log.total_tokens + '</td><td>' + log.cost.toFixed(6) + '</td><td>' + log.latency_ms + '</td><td>' + log.created_at + '</td></tr>';
        });
        html += '</tbody></table></div>';

        container.innerHTML = html;
    } catch (error) {
        console.error('Failed to load usage:', error);
    }
}

// Load Upstream Usage
async function loadUpstreamUsage() {
    try {
        const response = await adminFetch('/admin/upstream-usage');
        const result = await response.json();

        const container = document.getElementById('upstreamUsage');

        if (!result.data || result.data.length === 0) {
            container.innerHTML = '<p class="empty">暂无上游用量数据</p>';
            return;
        }

        var html = '';
        result.data.forEach(function(keyData, index) {
            var data = keyData;
            if (data.base_resp && data.base_resp.status_code !== 0) {
                html += '<div class="upstream-key-section"><h3>API Key ' + (index + 1) + '</h3><p class="empty">获取用量失败: ' + (data.base_resp.status_msg || '未知错误') + '</p></div>';
                return;
            }

            var modelRemains = data.model_remains || [];
            if (modelRemains.length === 0) {
                html += '<div class="upstream-key-section"><h3>API Key ' + (index + 1) + '</h3><p class="empty">暂无用量数据</p></div>';
                return;
            }

            html += '<div class="upstream-key-section"><h3>API Key ' + (index + 1) + '</h3><div class="table-wrapper"><table><thead><tr><th>模型</th><th>剩余时间</th><th>当前周期配额</th><th>剩余</th><th>剩余用量百分比</th></tr></thead><tbody>';

            modelRemains.forEach(function(m) {
                var usagePercent = m.current_interval_total_count > 0
                    ? (m.current_interval_usage_count / m.current_interval_total_count * 100).toFixed(1)
                    : 0;
                var remainsHours = (m.remains_time / 3600/1000).toFixed(1);
                var remaining = m.current_interval_total_count - m.current_interval_usage_count;

                html += '<tr><td>' + m.model_name + '</td><td>' + remainsHours + ' 小时</td><td>' + m.current_interval_total_count + '</td><td>' + m.current_interval_usage_count + '</td><td><div class="progress-bar"><div class="progress-fill" style="width: ' + usagePercent + '%"></div></div><span>' + usagePercent + '% (已用 ' + remaining + ')</span></td></tr>';
            });

            html += '</tbody></table></div></div>';
        });

        container.innerHTML = html;
    } catch (error) {
        console.error('Failed to load upstream usage:', error);
        document.getElementById('upstreamUsage').innerHTML = '<p class="empty">获取上游用量失败</p>';
    }
}

// Create User button
document.getElementById('createUserBtn').addEventListener('click', function() {
    var defaultExpiresAt = new Date();
    defaultExpiresAt.setDate(defaultExpiresAt.getDate() + 7);
    document.getElementById('createUserModal').querySelector('input[name="expires_at"]').value = defaultExpiresAt.toISOString().slice(0, 16);
    document.getElementById('createUserModal').style.display = 'block';
});

document.getElementById('createUserForm').addEventListener('submit', async function(e) {
    e.preventDefault();
    var formData = new FormData(e.target);

    var expiresAt = formData.get('expires_at');
    var requestBody = {
        username: formData.get('username'),
        password: formData.get('password'),
        remark: formData.get('remark'),
        request_limit: parseInt(formData.get('request_limit')),
        has_weekly_limit: parseInt(formData.get('has_weekly_limit'))
    };
    if (expiresAt) {
        requestBody.expires_at = new Date(expiresAt).toISOString();
    }

    try {
        var response = await adminFetch('/admin/users', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(requestBody)
        });

        if (response.ok) {
            alert('用户创建成功');
            document.getElementById('createUserModal').style.display = 'none';
            e.target.reset();
            loadUsers();
        } else {
            alert('创建失败');
        }
    } catch (error) {
        alert('网络错误');
    }
});

// Create API Key
document.getElementById('createApiKeyBtn').addEventListener('click', function() {
    document.getElementById('createApiKeyModal').style.display = 'block';
});

document.getElementById('createApiKeyForm').addEventListener('submit', async function(e) {
    e.preventDefault();
    var formData = new FormData(e.target);

    try {
        var response = await adminFetch('/admin/apikeys', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                user_id: parseInt(formData.get('user_id')),
                key_name: formData.get('key_name')
            })
        });

        if (response.ok) {
            alert('API Key 创建成功');
            document.getElementById('createApiKeyModal').style.display = 'none';
            e.target.reset();
            loadAPIKeys();
        } else {
            alert('创建失败');
        }
    } catch (error) {
        alert('网络错误');
    }
});

// Batch create activation users
document.getElementById('batchCreateActivationBtn').addEventListener('click', function() {
    document.getElementById('batchCreateActivationModal').style.display = 'block';
});

document.getElementById('batchCreateActivationForm').addEventListener('submit', async function(e) {
    e.preventDefault();
    var formData = new FormData(e.target);

    var password = formData.get('password');
    var validDays = parseInt(formData.get('valid_days'));
    var requestLimit = parseInt(formData.get('request_limit'));
    var count = parseInt(formData.get('count'));
    var level = parseInt(formData.get('level')) || 1;
    var useGml = parseInt(formData.get('use_gml')) || -1;
    var useKimi = parseInt(formData.get('use_kimi')) || -1;
    var hasWeeklyLimit = parseInt(formData.get('has_weekly_limit')) || -1;
    var remarks = formData.get('remarks') || '';

    if (count < 1 || count > 100) {
        alert('账号个数必须在1-100之间');
        return;
    }

    function generateUsername() {
        var chars = 'abcdefghijklmnopqrstuvwxyz';
        var result = '';
        for (var i = 0; i < 8; i++) {
            result += chars.charAt(Math.floor(Math.random() * chars.length));
        }
        return result;
    }

    var users = [];
    for (var i = 0; i < count; i++) {
        users.push({
            username: generateUsername(),
            password: password,
            valid_days: validDays,
            request_limit: requestLimit,
            level: level,
            use_gml: useGml,
            use_kimi: useKimi,
            has_weekly_limit: hasWeeklyLimit,
            remarks: remarks
        });
    }

    try {
        var response = await adminFetch('/admin/activation-users/batch', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ users: users })
        });

        if (response.ok) {
            var createdUsers = await response.json();
            alert('成功创建 ' + createdUsers.length + ' 个待激活用户');
            document.getElementById('batchCreateActivationModal').style.display = 'none';
            e.target.reset();
            loadActivationUsers();
        } else {
            var error = await response.json();
            alert('创建失败: ' + (error.error || '未知错误'));
        }
    } catch (error) {
        alert('网络错误');
    }
});

// Edit User
function editUser(id, currentLimit, expiresAt, username, remark, level, weeklyLimit, useGml, useKimi) {
    level = level || 1;
    weeklyLimit = weeklyLimit !== undefined ? weeklyLimit : -1;
    useGml = useGml !== undefined ? useGml : -1;
    useKimi = useKimi !== undefined ? useKimi : -1;
    document.getElementById('editUserId').value = id;
    document.getElementById('editUsername').value = username;
    document.getElementById('editRequestLimit').value = currentLimit;

    if (expiresAt) {
        document.getElementById('editExpiresAt').value = expiresAt.replace(' ', 'T').slice(0, 16);
    } else {
        document.getElementById('editExpiresAt').value = '';
    }

    document.getElementById('editRemark').value = remark || '';
    document.getElementById('editLevel').value = level;
    document.getElementById('editWeeklyLimit').value = weeklyLimit;
    document.getElementById('editUseGml').value = useGml;
    document.getElementById('editUseKimi').value = useKimi;

    document.getElementById('editUserModal').style.display = 'block';
}

document.getElementById('editUserForm').addEventListener('submit', async function(e) {
    e.preventDefault();
    var formData = new FormData(e.target);
    var userId = formData.get('id');

    var expiresAt = formData.get('expires_at');
    var requestBody = {
        request_limit: parseInt(formData.get('request_limit')),
        has_weekly_limit: parseInt(formData.get('has_weekly_limit'))
    };
    if (expiresAt) {
        requestBody.expires_at = new Date(expiresAt).toISOString();
    }
    requestBody.remark = formData.get('remark');
    requestBody.level = parseInt(formData.get('level'));
    requestBody.use_gml = parseInt(formData.get('use_gml'));
    requestBody.use_kimi = parseInt(formData.get('use_kimi'));

    try {
        var response = await adminFetch('/admin/users/' + userId, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(requestBody)
        });

        if (response.ok) {
            document.getElementById('editUserModal').style.display = 'none';
            loadUsers();
        } else {
            alert('更新失败');
        }
    } catch (error) {
        alert('网络错误');
    }
});

async function deleteUser(id) {
    if (!confirm('确定要删除这个用户吗？')) return;

    try {
        var response = await adminFetch('/admin/users/' + id, { method: 'DELETE' });
        if (response.ok) {
            loadUsers();
        } else {
            alert('删除失败');
        }
    } catch (error) {
        alert('网络错误');
    }
}

// API Key operations
async function resetApiKey(id) {
    if (!confirm('确定要重置这个 API Key 吗？')) return;

    try {
        var response = await adminFetch('/admin/apikeys/' + id + '/reset', { method: 'POST' });
        var data = await response.json();

        if (response.ok) {
            alert('API Key 已重置: ' + data.key_value);
            loadAPIKeys();
        } else {
            alert('重置失败');
        }
    } catch (error) {
        alert('网络错误');
    }
}

async function toggleApiKey(id) {
    try {
        var response = await adminFetch('/admin/apikeys/' + id + '/toggle', { method: 'POST' });
        if (response.ok) {
            loadAPIKeys();
        } else {
            alert('操作失败');
        }
    } catch (error) {
        alert('网络错误');
    }
}

async function deleteApiKey(id) {
    if (!confirm('确定要删除这个 API Key 吗？')) return;

    try {
        var response = await adminFetch('/admin/apikeys/' + id, { method: 'DELETE' });
        if (response.ok) {
            loadAPIKeys();
        } else {
            alert('删除失败');
        }
    } catch (error) {
        alert('网络错误');
    }
}

// Modal close
document.querySelectorAll('.close').forEach(function(close) {
    close.addEventListener('click', function() {
        document.querySelectorAll('.modal').forEach(function(m) {
            m.style.display = 'none';
        });
    });
});

// Logout
document.getElementById('logoutBtn').addEventListener('click', function() {
    localStorage.removeItem('user');
    localStorage.removeItem('token');
    window.location.href = '/login.html';
});

// Initial load
loadUsers();