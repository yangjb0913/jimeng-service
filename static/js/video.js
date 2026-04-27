document.addEventListener('DOMContentLoaded', function() {
    initFunctionChange();
    initUpload();
    initGenerate();
});

let uploadedImages = [];
let tailUploadedImages = [];

function initFunctionChange() {
    const functionSelect = document.getElementById('function-select');

    functionSelect.addEventListener('change', updateUI);

    updateUI();
}

function updateUI() {
    const functionSelect = document.getElementById('function-select');
    const value = functionSelect.value;
    const imageSection = document.getElementById('image-section');
    const tailImageSection = document.getElementById('tail-image-section');
    const cameraSection = document.getElementById('camera-section');

    imageSection.style.display = 'none';
    tailImageSection.style.display = 'none';
    cameraSection.style.display = 'none';

    if (value.startsWith('i2v_first')) {
        imageSection.style.display = 'block';
    } else if (value.startsWith('i2v_first_tail')) {
        imageSection.style.display = 'block';
        tailImageSection.style.display = 'block';
    }

    if (value === 'i2v_recamera_720') {
        cameraSection.style.display = 'block';
    }
}

function initUpload() {
    setupUploadArea('upload-area', 'image-input', uploadedImages, 'uploaded-images');

    const tailUploadArea = document.getElementById('tail-upload-area');
    const tailImageInput = document.getElementById('tail-image-input');

    tailUploadArea.addEventListener('click', () => tailImageInput.click());

    tailUploadArea.addEventListener('dragover', (e) => {
        e.preventDefault();
        tailUploadArea.classList.add('dragover');
    });

    tailUploadArea.addEventListener('dragleave', () => {
        tailUploadArea.classList.remove('dragover');
    });

    tailUploadArea.addEventListener('drop', (e) => {
        e.preventDefault();
        tailUploadArea.classList.remove('dragover');
        handleTailFiles(e.dataTransfer.files);
    });

    tailImageInput.addEventListener('change', () => {
        handleTailFiles(tailImageInput.files);
    });
}

function setupUploadArea(areaId, inputId, imagesArray, containerId) {
    const uploadArea = document.getElementById(areaId);
    const imageInput = document.getElementById(inputId);

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
        handleFiles(e.dataTransfer.files, imagesArray, containerId);
    });

    imageInput.addEventListener('change', () => {
        handleFiles(imageInput.files, imagesArray, containerId);
    });
}

async function handleFiles(files, imagesArray, containerId) {
    const container = document.getElementById(containerId);

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
                imagesArray.push(data.data.url);
                renderUploadedImage(data.data.url, container, imagesArray);
            }
        } catch (error) {
            showToast('上传失败: ' + error.message, 'error');
        }
    }
}

async function handleTailFiles(files) {
    await handleFiles(files, tailUploadedImages, 'tail-uploaded-images');
}

function renderUploadedImage(url, container, imagesArray) {
    const div = document.createElement('div');
    div.className = 'uploaded-image';
    div.innerHTML = `
        <img src="${url}" alt="上传图片">
        <button class="remove-btn" data-url="${url}">×</button>
    `;
    div.querySelector('.remove-btn').addEventListener('click', () => {
        const index = imagesArray.indexOf(url);
        if (index > -1) imagesArray.splice(index, 1);
        div.remove();
    });
    container.appendChild(div);
}

function initGenerate() {
    const generateBtn = document.getElementById('generate-btn');

    generateBtn.addEventListener('click', async () => {
        const functionSelect = document.getElementById('function-select');
        const prompt = document.getElementById('prompt').value.trim();
        const frames = parseInt(document.getElementById('duration-select').value);
        const aspectRatio = document.getElementById('aspect-ratio').value;
        const templateId = document.getElementById('camera-template')?.value;
        const cameraStrength = 'medium';

        if (!prompt) {
            showToast('请输入描述', 'error');
            return;
        }

        const func = functionSelect.value;

        if (func.startsWith('i2v_first') && uploadedImages.length === 0) {
            showToast('请上传参考图片', 'error');
            return;
        }

        if (func.startsWith('i2v_first_tail') && (uploadedImages.length === 0 || tailUploadedImages.length === 0)) {
            showToast('请上传首帧和尾帧参考图片', 'error');
            return;
        }

        const body = {
            function: func,
            prompt: prompt,
            frames: frames,
            aspect_ratio: aspectRatio,
        };

        if (func.startsWith('i2v_first') && !func.startsWith('i2v_first_tail')) {
            body.image_urls = uploadedImages;
        } else if (func.startsWith('i2v_first_tail')) {
            body.image_urls = [...uploadedImages, ...tailUploadedImages];
        }

        if (templateId) {
            body.template_id = templateId;
            body.camera_strength = cameraStrength;
        }

        generateBtn.disabled = true;
        generateBtn.textContent = '生成中...';

        try {
            const response = await fetch(`${API_BASE}/video/generate`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body),
            });
            const data = await response.json();

            if (data.code === 0) {
                showToast('任务已提交', 'success');
                pollTaskStatus(data.data.task_id, data.data.duration);
            } else {
                showToast(data.message || '提交失败', 'error');
                generateBtn.disabled = false;
                generateBtn.textContent = '🎬 开始生成';
            }
        } catch (error) {
            showToast('提交失败: ' + error.message, 'error');
            generateBtn.disabled = false;
            generateBtn.textContent = '🎬 开始生成';
        }
    });
}

function pollTaskStatus(taskId, duration) {
    const resultSection = document.getElementById('result-section');
    const statusText = document.getElementById('status-text');
    const durationText = document.getElementById('duration-text');
    const videoResult = document.getElementById('video-result');
    const generateBtn = document.getElementById('generate-btn');

    resultSection.style.display = 'block';
    statusText.textContent = '等待中...';
    durationText.textContent = duration ? `${duration}秒` : '-';
    videoResult.innerHTML = '';
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
                    if (data.data.result && data.data.result.video_url) {
                        videoResult.innerHTML = `
                            <video controls src="${data.data.result.video_url}">
                                您的浏览器不支持视频播放
                            </video>
                            <p><a href="${data.data.result.video_url}" download>下载视频</a></p>
                        `;
                    }
                    generateBtn.disabled = false;
                    generateBtn.textContent = '🎬 开始生成';
                } else if (status === 'failed') {
                    clearInterval(interval);
                    showToast(data.data.error || '生成失败', 'error');
                    generateBtn.disabled = false;
                    generateBtn.textContent = '🎬 开始生成';
                }
            }
        } catch (error) {
            console.error('Poll error:', error);
        }
    }, 3000);
}
