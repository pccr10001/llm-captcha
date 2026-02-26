const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('api', {
  connect: (url) => ipcRenderer.send('connect', url),
  disconnect: () => ipcRenderer.send('disconnect'),
  onStatusUpdate: (callback) => {
    ipcRenderer.on('status-update', (_event, status) => callback(status));
  },
  onTaskUpdate: (callback) => {
    ipcRenderer.on('task-update', (_event, tasks) => callback(tasks));
  },
});
