const { contextBridge, ipcRenderer } = require("electron")

contextBridge.exposeInMainWorld("admin", {
  getProfiles: () => ipcRenderer.invoke("admin:getProfiles"),
  saveProfile: (profile) => ipcRenderer.invoke("admin:saveProfile", profile),
  deleteProfile: (profileId) => ipcRenderer.invoke("admin:deleteProfile", profileId),
  setActiveProfile: (profileId) => ipcRenderer.invoke("admin:setActiveProfile", profileId),
  getActiveProfile: () => ipcRenderer.invoke("admin:getActiveProfile"),
  testConnection: (profileId) => ipcRenderer.invoke("admin:testConnection", profileId),
  apiGet: (moduleName, resourcePath, query) => ipcRenderer.invoke("admin:apiGet", moduleName, resourcePath, query),
  apiPost: (moduleName, resourcePath, payload) => ipcRenderer.invoke("admin:apiPost", moduleName, resourcePath, payload),
  getRecentOperations: () => ipcRenderer.invoke("admin:getRecentOperations"),
  readGameData: (tableName) => ipcRenderer.invoke("admin:readGameData", tableName)
})
