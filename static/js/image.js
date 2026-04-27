document.addEventListener('DOMContentLoaded', function() {
    initUpload();
    initUrlInput();
    initSizePresets();
    initScaleSlider();
    initGenerate();
});

let uploadedImages = [];

function initUrlInput() {
    const urlInput = document.getElementById('image-url-input');
    const addBtn = document.getElementById('add-url-btn');

    addBtn.addEventListener('click', () => {
        const url = urlInput.value.trim();
        if (url) {
            uploadedImages.push(url);
            renderUploadedImage(url, 'URL Image');
            urlInput.value = '';
        }
    });

    urlInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            addBtn.click();
        }
    });
}

function initUpload() {
    const uploadArea = document.getElementById('upload-area');
    const imageInput = document.getElementById('image-input');
    const uploadedImagesDiv = document.getElementById('uploaded-images');

    uploadArea.addEventListener('click', () => imageInput.click());

    uploadArea.addEventListener('dragover', (e) => {
        e.preventDefault();
        uploadArea.classList.add('dragover');
    });

    uploadArea.addEventListener('dragleave', () => {
        uploadArea.classList.remove('dragover');
    });

    uploadArea.addEventListener('drop', (e) => {
        e.preventDefault();
        uploadArea.classList.remove('dragover');
        handleFiles(e.dataTransfer.files);
    });

    imageInput.addEventListener('change', () => {
        handleFiles(imageInput.files);
    });
}

async function handleFiles(files) {
    const uploadedImagesDiv = document.getElementById('uploaded-images');

    for (const file of files) {
        if (!file.type.startsWith('image/')) continue;

        const formData = new FormData();
        formData.append('image', file);

        try {
            const response = await fetch(`${API_BASE}/upload`, {
                method: 'POST',
                body: formData,
            });
            const data = await response.json();
            if (data.code === 0) {
                uploadedImages.push(data.data.url);
                renderUploadedImage(data.data.url, file.name);
            }
        } catch (error) {
            showToast('上传失败: ' + error.message, 'error');
        }
    }
}

function renderUploadedImage(url, filename) {
    const uploadedImagesDiv = document.getElementById('uploaded-images');
    const div = document.createElement('div');
    div.className = 'uploaded-image';
    div.innerHTML = `
        <img src="${url}" alt="${filename}">
        <button class="remove-btn" data-url="${url}">×</button>
    `;
    div.querySelector('.remove-btn').addEventListener('click', () => {
        uploadedImages = uploadedImages.filter(u => u !== url);
        div.remove();
    });
    uploadedImagesDiv.appendChild(div);
}

async function initSizePresets() {
    try {
        const response = await fetch(`${API_BASE}/image/size-presets`);
        const data = await response.json();
        const select = document.getElementById('size-preset');

        data.data.forEach(preset => {
            const option = document.createElement('option');
            option.value = JSON.stringify({ width: preset.width, height: preset.height });
            option.textContent = `${preset.label} (${preset.width}x${preset.height})`;
            select.appendChild(option);
        });

        select.addEventListener('change', () => {
            if (select.value) {
                const { width, height } = JSON.parse(select.value);
                document.getElementById('width').value = width;
                document.getElementById('height').value = height;
            }
        });
    } catch (error) {
        console.error('Failed to load size presets:', error);
    }
}

function initScaleSlider() {
    const scale = document.getElementById('scale');
    const scaleValue = document.getElementById('scale-value');

    scale.addEventListener('input', () => {
        scaleValue.textContent = scale.value;
    });
}

function initGenerate() {
    const generateBtn = document.getElementById('generate-btn');

    generateBtn.addEventListener('click', async () => {
        const functionSelect = document.getElementById('function-select');
        const prompt = document.getElementById('prompt').value.trim();

        if (!prompt) {
            showToast('请输入描述', 'error');
            return;
        }

        const width = parseInt(document.getElementById('width').value) || 0;
        const height = parseInt(document.getElementById('height').value) || 0;
        const scale = parseInt(document.getElementById('scale').value) / 100;

        const body = {
            function: functionSelect.value,
            prompt: prompt,
            image_urls: uploadedImages,
        };

        if (width && height) {
            body.width = width;
            body.height = height;
        }

        if (scale && scale !== 0.5) {
            body.scale = scale;
        }

        generateBtn.disabled = true;
        generateBtn.textContent = '生成中...';

        try {
            const response = await fetch(`${API_BASE}/image/generate`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body),
            });
            const data = await response.json();

            if (data.code === 0) {
                showToast('任务已提交', 'success');
                pollTaskStatus(data.data.task_id);
            } else {
                showToast(data.message || '提交失败', 'error');
                generateBtn.disabled = false;
                generateBtn.textContent = '🎨 开始生成';
            }
        } catch (error) {
            showToast('提交失败: ' + error.message, 'error');
            generateBtn.disabled = false;
            generateBtn.textContent = '🎨 开始生成';
        }
    });
}

function pollTaskStatus(taskId) {
    const resultSection = document.getElementById('result-section');
    const statusText = document.getElementById('status-text');
    const imageResults = document.getElementById('image-results');
    const generateBtn = document.getElementById('generate-btn');

    resultSection.style.display = 'block';
    statusText.textContent = '等待中...';
    imageResults.innerHTML = '';
    const taskStatus = document.getElementById('task-status');
    taskStatus.className = 'task-status';

    const interval = setInterval(async () => {
        try {
            const response = await fetch(`${API_BASE}/task/result/${taskId}`);
            const data = await response.json();

            if (data.code === 0) {
                const status = data.data.status;
                statusText.textContent = getStatusText(status);
                taskStatus.className = 'task-status ' + status;

                if (status === 'done') {
                    clearInterval(interval);
                    if (data.data.result && data.data.result.image_urls) {
                        imageResults.innerHTML = data.data.result.image_urls.map((url, idx) =>
                            `<div class="image-result-item">
                                <img src="${url}" alt="生成结果">
                                <div class="preview-overlay">
                                    <div class="icon-btn" onclick="viewImage('${url}')">
                                        <svg viewBox="0 0 24 24" fill="none" stroke="#333" stroke-width="2">
                                            <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/>
                                            <circle cx="12" cy="12" r="3"/>
                                        </svg>
                                    </div>
                                    <div class="icon-btn" onclick="downloadImage('${url}', ${idx})">
                                        <svg viewBox="0 0 24 24" fill="none" stroke="#333" stroke-width="2">
                                            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
                                            <polyline points="7 10 12 15 17 10"/>
                                            <line x1="12" y1="15" x2="12" y2="3"/>
                                        </svg>
                                    </div>
                                </div>
                            </div>`
                        ).join('');
                    }
                    generateBtn.disabled = false;
                    generateBtn.textContent = '🎨 开始生成';
                } else if (status === 'failed') {
                    clearInterval(interval);
                    showToast(data.data.error || '生成失败', 'error');
                    generateBtn.disabled = false;
                    generateBtn.textContent = '🎨 开始生成';
                }
            }
        } catch (error) {
            console.error('Poll error:', error);
        }
    }, 3000);
}

function viewImage(url) {
    document.getElementById('modal-image').src = url;
    document.getElementById('image-modal').classList.add('active');
}

function closeModal(event) {
    if (!event || event.target.classList.contains('modal') || event.target.classList.contains('modal-close')) {
        document.getElementById('image-modal').classList.remove('active');
    }
}

function downloadImage(url, idx) {
    fetch(url)
        .then(resp => resp.blob())
        .then(blob => {
            const a = document.createElement('a');
            a.href = URL.createObjectURL(blob);
            a.download = `generated_image_${Date.now()}_${idx}.png`;
            a.click();
            URL.revokeObjectURL(a.href);
        })
        .catch(err => showToast('下载失败', 'error'));
}
