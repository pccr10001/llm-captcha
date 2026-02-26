const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('captchaBridge', {
  solved: (token) => ipcRenderer.send('captcha-solved', token),
});
