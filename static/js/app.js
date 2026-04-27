document.addEventListener('DOMContentLoaded', function() {
    loadRecentTasks();
    setInterval(loadRecentTasks, 10000);
});

async function loadRecentTasks() {
    const taskList = document.getElementById('task-list');
    if (!taskList) return;

    try {
        const response = await fetch(`${API_BASE}/task/list?limit=5&offset=0`);
        const data = await response.json();

        if (data.code !== 0 || !data.data.tasks || data.data.tasks.length === 0) {
            taskList.innerHTML = '<p class="loading">暂无任务记录</p>';
            return;
        }

        taskList.innerHTML = data.data.tasks.map(task => {
            const imageHtml = getTaskThumbnail(task);
            return `
                <div class="task-item" onclick="location.href='/tasks.html'">
                    ${imageHtml}
                    <div class="task-info">
                        <span class="task-type">${task.type === 'image' ? '📷 生图' : '🎬 生视频'}</span>
                        <p class="task-prompt">${task.prompt || '-'}</p>
                    </div>
                    <span class="task-status ${getStatusClass(task.status)}">${getStatusText(task.status)}</span>
                </div>
            `;
        }).join('');
    } catch (error) {
        console.error('Failed to load tasks:', error);
        taskList.innerHTML = '<p class="loading">加载失败</p>';
    }
}

function getTaskThumbnail(task) {
    if (task.status === 'failed') {
        return '<div class="task-thumb placeholder">❌</div>';
    }

    if (task.type === 'image' && task.result && task.result.image_urls && task.result.image_urls.length > 0) {
        return `<img class="task-thumb" src="${task.result.image_urls[0]}" alt="生成结果">`;
    }

    if (task.type === 'video' && task.result && task.result.video_url) {
        return `<video class="task-thumb" src="${task.result.video_url}" muted></video>`;
    }

    return '<div class="task-thumb placeholder">⏳</div>';
}
