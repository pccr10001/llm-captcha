const serverUrlInput = document.getElementById('serverUrl');
const connectBtn = document.getElementById('connectBtn');
const statusDot = document.getElementById('statusDot');
const statusText = document.getElementById('statusText');
const taskList = document.getElementById('taskList');

let connected = false;

connectBtn.addEventListener('click', () => {
  if (connected) {
    window.api.disconnect();
  } else {
    const url = serverUrlInput.value.trim();
    if (url) {
      window.api.connect(url);
    }
  }
});

window.api.onStatusUpdate((status) => {
  statusDot.className = 'status-dot';
  connectBtn.className = '';

  switch (status) {
    case 'connected':
      connected = true;
      statusDot.classList.add('connected');
      statusText.textContent = 'Connected';
      connectBtn.textContent = 'Disconnect';
      connectBtn.classList.add('disconnect');
      serverUrlInput.disabled = true;
      break;
    case 'connecting':
      connected = false;
      statusDot.classList.add('connecting');
      statusText.textContent = 'Connecting...';
      connectBtn.textContent = 'Connect';
      serverUrlInput.disabled = true;
      break;
    case 'disconnected':
      connected = false;
      statusText.textContent = 'Disconnected';
      connectBtn.textContent = 'Connect';
      serverUrlInput.disabled = false;
      break;
  }
});

window.api.onTaskUpdate((tasks) => {
  if (tasks.length === 0) {
    taskList.innerHTML = '<div class="empty-state">No tasks yet. Connect to the server to receive captcha tasks.</div>';
    return;
  }

  taskList.innerHTML = tasks.map((t) => {
    const shortId = t.taskId.substring(0, 8);
    let hostname = '';
    try { hostname = new URL(t.websiteURL).hostname; } catch { hostname = t.websiteURL || 'N/A'; }

    const details = [];
    if (t.websiteURL) details.push({ label: 'URL', value: t.websiteURL });
    if (t.websiteKey) details.push({ label: 'Key', value: t.websiteKey });
    if (t.isInvisible) details.push({ label: 'Invisible', value: 'Yes' });

    const detailsHtml = details.map(d =>
      `<div class="task-detail-row"><span class="task-detail-label">${d.label}</span><span class="task-detail-value">${d.value}</span></div>`
    ).join('');

    return `
      <div class="task-item" onclick="this.classList.toggle('expanded')">
        <div class="task-header">
          <div class="task-left">
            <span class="task-arrow">▶</span>
            <span class="task-type">${t.taskType || 'Unknown'}</span>
            <span class="task-id">${shortId}...</span>
            <span style="color:#585b70;font-size:11px">${hostname}</span>
          </div>
          <span class="task-status ${t.status}">${t.status}</span>
        </div>
        <div class="task-details">${detailsHtml}</div>
      </div>`;
  }).join('');
});
