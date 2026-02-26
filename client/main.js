const { app, BrowserWindow, ipcMain, session, net } = require('electron');
const path = require('path');
const WebSocket = require('ws');

let settingsWin = null;
let ws = null;
let reconnectTimer = null;
let reconnectDelay = 1000;
const MAX_RECONNECT_DELAY = 30000;
let serverUrl = '';

// taskId -> { window, task, status }
const activeTasks = new Map();

// Shared persistent sessions per captcha provider (cookies accumulate for trust)
// partition -> { session, protocolRegistered, pendingPages: Map<taskId, {html, allowedHosts}> }
const sharedSessions = new Map();

function getSessionPartition(taskType) {
  if (taskType === 'RecaptchaV2Task' || taskType === 'RecaptchaV3Task') return 'persist:recaptcha';
  if (taskType === 'GeeTestTask') return 'persist:geetest';
  return 'persist:default';
}

function getOrCreateSharedSession(partition) {
  if (sharedSessions.has(partition)) return sharedSessions.get(partition);

  const ses = session.fromPartition(partition);
  const entry = { session: ses, protocolRegistered: false, pendingPages: new Map() };
  sharedSessions.set(partition, entry);
  return entry;
}

function createSettingsWindow() {
  settingsWin = new BrowserWindow({
    width: 600,
    height: 500,
    title: 'LLM Captcha Client',
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });

  settingsWin.loadFile(path.join(__dirname, 'settings.html'));
  settingsWin.on('closed', () => { settingsWin = null; });
}

app.whenReady().then(() => {
  createSettingsWindow();
  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) createSettingsWindow();
  });
});

app.on('window-all-closed', () => {
  disconnectWs();
  app.quit();
});

// ---- WebSocket ----

function sendStatusUpdate(status) {
  if (settingsWin && !settingsWin.isDestroyed()) {
    settingsWin.webContents.send('status-update', status);
  }
}

function sendTaskUpdate() {
  if (settingsWin && !settingsWin.isDestroyed()) {
    const tasks = [];
    for (const [taskId, entry] of activeTasks) {
      let params = {};
      try {
        const raw = entry.task.params;
        params = typeof raw === 'string' ? JSON.parse(raw) : (raw || {});
      } catch {}
      tasks.push({
        taskId,
        status: entry.status,
        taskType: entry.task.taskType,
        websiteURL: params.websiteURL || '',
        websiteKey: params.websiteKey || params.gt || '',
        isInvisible: params.isInvisible || false,
      });
    }
    settingsWin.webContents.send('task-update', tasks);
  }
}

function connectWs(url) {
  if (ws) disconnectWs();

  serverUrl = url;
  sendStatusUpdate('connecting');

  try {
    ws = new WebSocket(url);
  } catch (err) {
    sendStatusUpdate('disconnected');
    scheduleReconnect();
    return;
  }

  ws.on('open', () => {
    reconnectDelay = 1000;
    sendStatusUpdate('connected');
  });

  ws.on('message', (data) => {
    let msg;
    try { msg = JSON.parse(data.toString()); } catch { return; }

    if (msg.type === 'ping') {
      wsSend({ type: 'pong' });
      return;
    }
    if (msg.type === 'task') {
      handleTask(msg);
    }
  });

  ws.on('close', () => {
    ws = null;
    sendStatusUpdate('disconnected');
    scheduleReconnect();
  });

  ws.on('error', () => {});
}

function disconnectWs() {
  if (reconnectTimer) { clearTimeout(reconnectTimer); reconnectTimer = null; }
  serverUrl = '';
  if (ws) { ws.removeAllListeners(); ws.close(); ws = null; }
  sendStatusUpdate('disconnected');
}

function scheduleReconnect() {
  if (!serverUrl) return;
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    if (serverUrl) connectWs(serverUrl);
  }, reconnectDelay);
  reconnectDelay = Math.min(reconnectDelay * 2, MAX_RECONNECT_DELAY);
}

function wsSend(obj) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(obj));
  }
}

// ---- IPC from settings renderer ----
ipcMain.on('connect', (_event, url) => { connectWs(url); });
ipcMain.on('disconnect', () => { disconnectWs(); });

// ---- IPC from captcha renderer ----
ipcMain.on('captcha-solved', (event, solutionJSON) => {
  const senderWc = event.sender;
  for (const [taskId, entry] of activeTasks) {
    if (entry.window && !entry.window.isDestroyed() && entry.window.webContents === senderWc) {
      let solution;
      try { solution = JSON.parse(solutionJSON); } catch { solution = solutionJSON; }
      wsSend({ type: 'result', taskId, solution });
      entry.status = 'solved';
      sendTaskUpdate();
      setTimeout(() => {
        if (entry.window && !entry.window.isDestroyed()) entry.window.close();
      }, 1500);
      return;
    }
  }
});

// ---- Captcha HTML generators ----

function getRecaptchaV2HTML(params) {
  const { websiteKey, isInvisible, recaptchaDataSValue, apiDomain } = params;
  const domain = apiDomain ? `www.${apiDomain.replace(/^www\./, '')}` : 'www.google.com';
  const dataSAttr = recaptchaDataSValue ? ` data-s="${recaptchaDataSValue}"` : '';

  return `<!DOCTYPE html>
<html>
<head>
  <title>Solve Captcha</title>
  <style>
    body { display:flex; justify-content:center; align-items:center; min-height:100vh; margin:0; background:#f5f5f5; font-family:Arial,sans-serif; }
    .container { text-align:center; padding:20px; }
    .status { margin-top:15px; font-size:14px; color:#666; }
    .success { color:#4CAF50; font-weight:bold; }
    .error { color:#f44336; }
  </style>
</head>
<body>
  <div class="container">
    <h3>Please solve the captcha</h3>
    <div id="captcha-container"></div>
    <div class="status" id="status">Loading reCAPTCHA...</div>
  </div>
  <script>
    function onCaptchaSolved(token) {
      document.getElementById('status').className = 'status success';
      document.getElementById('status').textContent = 'Captcha solved! Sending response...';
      if (window.captchaBridge) window.captchaBridge.solved(JSON.stringify({ gRecaptchaResponse: token }));
    }

    var onRecaptchaLoad = function() {
      try {
        var widgetId = grecaptcha.render('captcha-container', {
          sitekey: '${websiteKey}',
          callback: onCaptchaSolved,
          ${isInvisible ? "size: 'invisible'," : ''}
        });
        document.getElementById('status').textContent = 'Captcha loaded. Please solve it.';
        ${isInvisible ? 'grecaptcha.execute(widgetId);' : ''}
      } catch(e) {
        document.getElementById('status').className = 'status error';
        document.getElementById('status').textContent = 'Error: ' + e.message;
      }
    };
  </script>
  <script src="https://${domain}/recaptcha/api.js?onload=onRecaptchaLoad&render=explicit"${dataSAttr}></script>
</body>
</html>`;
}

function getRecaptchaV3HTML(params) {
  const { websiteKey, pageAction, isEnterprise, apiDomain, minScore } = params;
  const domain = apiDomain || 'www.google.com';
  const scriptFile = isEnterprise ? 'enterprise.js' : 'api.js';
  const executeCall = isEnterprise
    ? `grecaptcha.enterprise.execute('${websiteKey}', {action: '${pageAction || 'verify'}'}).then(onCaptchaSolved);`
    : `grecaptcha.execute('${websiteKey}', {action: '${pageAction || 'verify'}'}).then(onCaptchaSolved);`;

  return `<!DOCTYPE html>
<html>
<head>
  <title>Solve reCAPTCHA v3</title>
  <script src="https://${domain}/recaptcha/${scriptFile}?render=${websiteKey}" async defer></script>
  <style>
    body { display:flex; justify-content:center; align-items:center; min-height:100vh; margin:0; background:#f5f5f5; font-family:Arial,sans-serif; }
    .container { text-align:center; padding:20px; }
    .status { margin-top:15px; font-size:14px; color:#666; }
    .success { color:#4CAF50; font-weight:bold; }
  </style>
</head>
<body>
  <div class="container">
    <h3>reCAPTCHA v3 (min score: ${minScore})</h3>
    <p>Executing automatically...</p>
    <div class="status" id="status">Loading reCAPTCHA v3...</div>
  </div>
  <script>
    function onCaptchaSolved(token) {
      document.getElementById('status').className = 'status success';
      document.getElementById('status').textContent = 'Token obtained!';
      if (window.captchaBridge) window.captchaBridge.solved(JSON.stringify({ gRecaptchaResponse: token }));
    }
    ${isEnterprise ? 'if (typeof grecaptcha !== "undefined" && grecaptcha.enterprise) { grecaptcha.enterprise.ready(function() {' : 'if (typeof grecaptcha !== "undefined") { grecaptcha.ready(function() {'}
      ${executeCall}
    }); } else {
      window.addEventListener('load', function() {
        setTimeout(function() {
          ${isEnterprise ? 'grecaptcha.enterprise.ready(function() {' : 'grecaptcha.ready(function() {'}
            ${executeCall}
          });
        }, 1000);
      });
    }
  </script>
</body>
</html>`;
}

function getGeeTestHTML(params) {
  const { gt, challenge, geetestApiServerSubdomain, version, initParameters } = params;
  const v = version || 3;
  const apiSubdomain = geetestApiServerSubdomain || 'api.geetest.com';

  if (v === 4) {
    const initParams = initParameters || {};
    const captchaId = initParams.captcha_id || gt;
    return `<!DOCTYPE html>
<html>
<head>
  <title>Solve GeeTest v4</title>
  <script src="https://static.geetest.com/v4/gt4.js"></script>
  <style>
    body { display:flex; justify-content:center; align-items:center; min-height:100vh; margin:0; background:#f5f5f5; font-family:Arial,sans-serif; }
    .container { text-align:center; padding:20px; }
    .status { margin-top:15px; font-size:14px; color:#666; }
    .success { color:#4CAF50; font-weight:bold; }
  </style>
</head>
<body>
  <div class="container">
    <h3>Please solve the GeeTest v4 captcha</h3>
    <div id="captcha"></div>
    <div class="status" id="status">Loading GeeTest v4...</div>
  </div>
  <script>
    initGeetest4({
      captchaId: '${captchaId}',
      product: 'bind',
      ${Object.entries(initParams).filter(([k]) => k !== 'captcha_id').map(([k, v]) => `${k}: ${JSON.stringify(v)}`).join(',\n      ')}
    }, function(captchaObj) {
      captchaObj.appendTo('#captcha');
      captchaObj.onSuccess(function() {
        var result = captchaObj.getValidate();
        document.getElementById('status').className = 'status success';
        document.getElementById('status').textContent = 'GeeTest solved!';
        if (window.captchaBridge) window.captchaBridge.solved(JSON.stringify(result));
      });
    });
  </script>
</body>
</html>`;
  }

  // GeeTest v3
  return `<!DOCTYPE html>
<html>
<head>
  <title>Solve GeeTest v3</title>
  <script src="https://${apiSubdomain}/gettype.php?gt=${gt}&callback=geetestCallback"></script>
  <script src="https://static.geetest.com/static/js/gt.0.5.0.js"></script>
  <style>
    body { display:flex; justify-content:center; align-items:center; min-height:100vh; margin:0; background:#f5f5f5; font-family:Arial,sans-serif; }
    .container { text-align:center; padding:20px; }
    .status { margin-top:15px; font-size:14px; color:#666; }
    .success { color:#4CAF50; font-weight:bold; }
  </style>
</head>
<body>
  <div class="container">
    <h3>Please solve the GeeTest captcha</h3>
    <div id="captcha"></div>
    <div class="status" id="status">Loading GeeTest...</div>
  </div>
  <script>
    initGeetest({
      gt: '${gt}',
      challenge: '${challenge}',
      offline: false,
      new_captcha: true,
      product: 'bind',
      api_server: '${apiSubdomain}'
    }, function(captchaObj) {
      captchaObj.appendTo('#captcha');
      captchaObj.onSuccess(function() {
        var result = captchaObj.getValidate();
        document.getElementById('status').className = 'status success';
        document.getElementById('status').textContent = 'GeeTest solved!';
        if (window.captchaBridge) window.captchaBridge.solved(JSON.stringify({
          challenge: result.geetest_challenge,
          validate: result.geetest_validate,
          seccode: result.geetest_seccode
        }));
      });
    });
  </script>
</body>
</html>`;
}

// ---- Captcha task handling ----

function getAllowedHosts(taskType, params) {
  const base = ['fonts.googleapis.com', 'fonts.gstatic.com'];

  if (taskType === 'RecaptchaV2Task' || taskType === 'RecaptchaV3Task') {
    const domain = params.apiDomain || 'google.com';
    return [
      ...base,
      'www.google.com', 'google.com',
      'www.gstatic.com', 'ssl.gstatic.com',
      'www.recaptcha.net', 'recaptcha.net',
      'recaptcha.google.com',
      'apis.google.com',
      domain, `www.${domain}`,
    ];
  }

  if (taskType === 'GeeTestTask') {
    const apiSub = params.geetestApiServerSubdomain || 'api.geetest.com';
    return [
      ...base,
      'static.geetest.com', 'api.geetest.com',
      'gcaptcha4.geetest.com',
      apiSub,
    ];
  }

  return base;
}

function getCaptchaHTML(taskType, params) {
  switch (taskType) {
    case 'RecaptchaV2Task':
      return getRecaptchaV2HTML(params);
    case 'RecaptchaV3Task':
      return getRecaptchaV3HTML(params);
    case 'GeeTestTask':
      return getGeeTestHTML(params);
    default:
      return `<html><body><h3>Unsupported task type: ${taskType}</h3></body></html>`;
  }
}

function handleTask(task) {
  const { taskId, taskType, params: paramsRaw } = task;
  let params;
  try { params = typeof paramsRaw === 'string' ? JSON.parse(paramsRaw) : paramsRaw; } catch { params = {}; }

  const partition = getSessionPartition(taskType);
  const shared = getOrCreateSharedSession(partition);
  const ses = shared.session;

  const allowedHosts = getAllowedHosts(taskType, params);
  const captchaHTML = getCaptchaHTML(taskType, params);

  // Set custom user agent if provided
  if (params.userAgent) {
    ses.setUserAgent(params.userAgent);
  }

  // Set cookies if provided (format: key1=val1; key2=val2)
  if (params.cookies && params.websiteURL) {
    const url = new URL(params.websiteURL);
    const cookiePairs = params.cookies.split(';').map(c => c.trim()).filter(Boolean);
    for (const pair of cookiePairs) {
      const eqIdx = pair.indexOf('=');
      if (eqIdx > 0) {
        const name = pair.substring(0, eqIdx).trim();
        const value = pair.substring(eqIdx + 1).trim();
        ses.cookies.set({ url: url.origin, name, value }).catch(() => {});
      }
    }
  }

  // Register protocol handlers once per shared session
  if (!shared.protocolRegistered) {
    shared.protocolRegistered = true;

    ses.protocol.handle('https', (request) => {
      const url = new URL(request.url);

      // Serve initial captcha page: match by unique path /__captcha__/{taskId}
      if (url.pathname.startsWith('/__captcha__/')) {
        const tid = url.pathname.split('/')[2];
        const page = shared.pendingPages.get(tid);
        if (page) {
          shared.pendingPages.delete(tid);
          return new Response(page.html, { headers: { 'Content-Type': 'text/html; charset=utf-8' } });
        }
      }

      // Rebuild allowed hosts from all active tasks in this partition
      const allAllowed = new Set();
      for (const [, entry] of activeTasks) {
        if (getSessionPartition(entry.task.taskType) === partition) {
          let p;
          try { p = typeof entry.task.params === 'string' ? JSON.parse(entry.task.params) : (entry.task.params || {}); } catch { p = {}; }
          for (const h of getAllowedHosts(entry.task.taskType, p)) allAllowed.add(h);
        }
      }

      if (allAllowed.has(url.hostname)) {
        return net.fetch(request);
      }

      return new Response('', { status: 204 });
    });

    ses.protocol.handle('http', (request) => {
      const url = new URL(request.url);
      if (url.pathname.startsWith('/__captcha__/')) {
        const tid = url.pathname.split('/')[2];
        const page = shared.pendingPages.get(tid);
        if (page) {
          shared.pendingPages.delete(tid);
          return new Response(page.html, { headers: { 'Content-Type': 'text/html; charset=utf-8' } });
        }
      }
      return new Response('', { status: 204 });
    });
  }

  // Register this task's page for interception
  shared.pendingPages.set(taskId, { html: captchaHTML, allowedHosts });

  const captchaWin = new BrowserWindow({
    width: 450,
    height: 650,
    title: `{${taskType || 'Unknown'}:${params.websiteURL || 'N/A'}}`,
    webPreferences: {
      preload: path.join(__dirname, 'captcha-preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
      session: ses,
    },
  });

  activeTasks.set(taskId, { window: captchaWin, task, status: 'solving' });
  sendTaskUpdate();

  // Load with unique path so protocol handler can identify which task's HTML to serve
  // Origin stays correct (websiteURL's host) for reCAPTCHA domain validation
  const baseURL = params.websiteURL || 'https://captcha.local';
  const origin = new URL(baseURL).origin;
  captchaWin.loadURL(`${origin}/__captcha__/${taskId}`);

  captchaWin.on('closed', () => {
    shared.pendingPages.delete(taskId);
    const entry = activeTasks.get(taskId);
    if (entry && entry.status !== 'solved') {
      wsSend({ type: 'error', taskId, error: 'Window closed by user before solving' });
      entry.status = 'cancelled';
      entry.window = null;
      sendTaskUpdate();
    } else if (entry) {
      entry.window = null;
    }
  });
}
