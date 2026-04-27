document.addEventListener('DOMContentLoaded', function() {
    initModals();
    loadKeys();
    initImport();
});

async function loadKeys() {
    const keysList = document.getElementById('keys-list');
    if (!keysList) return;

    try {
        const response = await fetch(`${API_BASE}/keys`);
        const data = await response.json();

        if (data.code !== 0 || !data.data || data.data.length === 0) {
            keysList.innerHTML = '<p class="loading">暂无密钥，请添加</p>';
            return;
        }

        keysList.innerHTML = data.data.map(key => renderKeyCard(key)).join('');
        attachKeyEventListeners();
    } catch (error) {
        console.error('Failed to load keys:', error);
        keysList.innerHTML = '<p class="loading">加载失败</p>';
    }
}

function renderKeyCard(key) {
    const funcTags = Object.entries(key.functions || {}).map(([func, enabled]) => {
        const funcNames = {
            't2i_v40': '文生图4.0',
            't2i_46': '生图4.6',
            't2v_720': '文生视频720p',
            't2v_1080': '文生视频1080p',
            'i2v_first_720': '首帧视频720p',
            'i2v_first_1080': '首帧视频1080p',
            'i2v_first_tail_720': '首尾帧720p',
            'i2v_first_tail_1080': '首尾帧1080p',
            'i2v_recamera_720': '运镜视频',
            'ti2v_pro': '3.0Pro',
        };
        return `<span class="func-tag ${enabled ? 'enabled' : 'disabled'}">${funcNames[func] || func}</span>`;
    }).join('');

    const videoQuota = key.quotas?.video || { limit: 50, used: 0, enabled: true };
    const imageQuota = key.quotas?.image || { limit: 200, used: 0, enabled: true };

    const videoPercent = videoQuota.enabled ? (videoQuota.used / videoQuota.limit * 100) : 0;
    const imagePercent = imageQuota.enabled ? (imageQuota.used / imageQuota.limit * 100) : 0;

    return `
        <div class="key-card" data-id="${key.id}">
            <div class="key-card-header">
                <h3>🔑 ${key.name}</h3>
                <div class="key-actions">
                    <span class="key-status ${key.enabled ? 'enabled' : 'disabled'}">${key.enabled ? '正常' : '已禁用'}</span>
                    <button class="btn btn-secondary btn-sm" onclick="openEditModal('${key.id}')">编辑</button>
                    <button class="btn btn-secondary btn-sm" onclick="resetKey('${key.id}')">重置</button>
                    <button class="btn btn-delete btn-sm" onclick="deleteKey('${key.id}')">删除</button>
                </div>
            </div>
            <div class="key-stats">
                <span>权重: ${key.weight}</span>
                <span>失败次数: ${key.failed_count || 0}</span>
                <span>最后使用: ${formatTime(key.last_used)}</span>
            </div>
            <div class="key-functions">
                ${funcTags}
            </div>
            <div class="quotas">
                <div class="quota-item">
                    <span class="quota-label">视频配额</span>
                    <div class="quota-bar">
                        <div class="quota-fill ${videoPercent > 80 ? 'danger' : videoPercent > 50 ? 'warning' : ''}" style="width: ${videoPercent}%"></div>
                    </div>
                    <span class="quota-text">${videoQuota.used}/${videoQuota.limit}秒</span>
                </div>
                <div class="quota-item">
                    <span class="quota-label">图片配额</span>
                    <div class="quota-bar">
                        <div class="quota-fill ${imagePercent > 80 ? 'danger' : imagePercent > 50 ? 'warning' : ''}" style="width: ${imagePercent}%"></div>
                    </div>
                    <span class="quota-text">${imageQuota.used}/${imageQuota.limit}张</span>
                </div>
            </div>
        </div>
    `;
}

function attachKeyEventListeners() {
    document.querySelectorAll('.edit-key-btn').forEach(btn => {
        btn.addEventListener('click', () => openEditModal(btn.dataset.id));
    });

    document.querySelectorAll('.reset-key-btn').forEach(btn => {
        btn.addEventListener('click', () => resetKey(btn.dataset.id));
    });
}

async function resetKey(keyId) {
    if (!confirm('确定要重置此密钥吗？这将恢复所有功能和配额。')) return;

    try {
        const response = await fetch(`${API_BASE}/keys/${keyId}/reset`, { method: 'POST' });
        const data = await response.json();
        if (data.code === 0) {
            showToast('重置成功', 'success');
            loadKeys();
        } else {
            showToast(data.message || '重置失败', 'error');
        }
    } catch (error) {
        showToast('重置失败: ' + error.message, 'error');
    }
}

async function deleteKey(keyId) {
    if (!confirm('确定要删除此密钥吗？此操作不可撤销。')) return;

    try {
        const response = await fetch(`${API_BASE}/keys/${keyId}`, { method: 'DELETE' });
        const data = await response.json();
        if (data.code === 0) {
            showToast('删除成功', 'success');
            loadKeys();
        } else {
            showToast(data.message || '删除失败', 'error');
        }
    } catch (error) {
        showToast('删除失败: ' + error.message, 'error');
    }
}

function initModals() {
    const addModal = document.getElementById('add-key-modal');
    const editModal = document.getElementById('edit-key-modal');

    document.getElementById('add-key-btn').addEventListener('click', () => {
        addModal.classList.add('active');
    });

    document.querySelectorAll('.modal-close').forEach(btn => {
        btn.addEventListener('click', () => {
            document.querySelectorAll('.modal').forEach(m => m.classList.remove('active'));
        });
    });

    document.getElementById('add-key-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const form = e.target;
        const data = {
            ak: form.ak.value,
            sk: form.sk.value,
            name: form.name.value,
            weight: parseInt(form.weight.value) || 10,
        };

        try {
            const response = await fetch(`${API_BASE}/keys`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data),
            });
            const result = await response.json();
            if (result.code === 0) {
                showToast('添加成功', 'success');
                addModal.classList.remove('active');
                form.reset();
                loadKeys();
            } else {
                showToast(result.message || '添加失败', 'error');
            }
        } catch (error) {
            showToast('添加失败: ' + error.message, 'error');
        }
    });

    document.getElementById('edit-key-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const keyId = document.getElementById('edit-key-id').value;
        const data = {
            name: document.getElementById('edit-name').value,
            weight: parseInt(document.getElementById('edit-weight').value) || 10,
            quotas: {
                video: { limit: parseInt(document.getElementById('edit-video-limit').value) || 50, enabled: true },
                image: { limit: parseInt(document.getElementById('edit-image-limit').value) || 200, enabled: true },
            },
        };

        try {
            const response = await fetch(`${API_BASE}/keys/${keyId}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data),
            });
            const result = await response.json();
            if (result.code === 0) {
                showToast('保存成功', 'success');
                editModal.classList.remove('active');
                loadKeys();
            } else {
                showToast(result.message || '保存失败', 'error');
            }
        } catch (error) {
            showToast('保存失败: ' + error.message, 'error');
        }
    });

    document.getElementById('refresh-btn').addEventListener('click', loadKeys);
}

async function openEditModal(keyId) {
    try {
        const response = await fetch(`${API_BASE}/keys`);
        const data = await response.json();
        const key = data.data.find(k => k.id === keyId);

        if (key) {
            document.getElementById('edit-key-id').value = key.id;
            document.getElementById('edit-ak').value = key.ak;
            document.getElementById('edit-sk').value = key.sk;
            document.getElementById('edit-name').value = key.name;
            document.getElementById('edit-weight').value = key.weight;
            document.getElementById('edit-video-limit').value = key.quotas?.video?.limit || 50;
            document.getElementById('edit-image-limit').value = key.quotas?.image?.limit || 200;
            document.getElementById('edit-key-modal').classList.add('active');
        }
    } catch (error) {
        showToast('加载失败: ' + error.message, 'error');
    }
}

function initImport() {
    const importModal = document.getElementById('import-modal');

    document.getElementById('import-btn').addEventListener('click', () => {
        importModal.classList.add('active');
    });

    document.getElementById('import-submit').addEventListener('click', async () => {
        const importData = document.getElementById('import-data').value;

        try {
            const jsonData = JSON.parse(importData);
            const response = await fetch(`${API_BASE}/keys/import`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(jsonData),
            });
            const result = await response.json();
            if (result.code === 0) {
                showToast(`导入成功，共${result.count}个密钥`, 'success');
                importModal.classList.remove('active');
                document.getElementById('import-data').value = '';
                loadKeys();
            } else {
                showToast(result.message || '导入失败', 'error');
            }
        } catch (error) {
            showToast('JSON格式错误: ' + error.message, 'error');
        }
    });
}
