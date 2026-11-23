const { app, BrowserWindow, ipcMain } = require("electron")
const path = require("path")
const fs = require("fs")

const adminModulePrefixes = {
  risk: "/api/admin/risk",
  users: "/api/admin/users",
  release: "/api/admin/release",
  battle: "/api/admin/battle",
  inventory: "/api/admin/inventory",
  cattery: "/api/admin/cattery",
  wardrobe: "/api/admin/wardrobe"
}

const recentOperations = []
const maxRecentOperations = 100

function configFilePath() {
  return path.join(app.getPath("userData"), "admin-client-config.json")
}

function loadConfig() {
  const file = configFilePath()
  if (!fs.existsSync(file)) {
    return { activeProfileId: "", profiles: [] }
  }
  try {
    return JSON.parse(fs.readFileSync(file, "utf8"))
  } catch {
    return { activeProfileId: "", profiles: [] }
  }
}

function saveConfig(config) {
  fs.mkdirSync(path.dirname(configFilePath()), { recursive: true })
  fs.writeFileSync(configFilePath(), JSON.stringify(config, null, 2))
}

function normalizeProfile(input) {
  return {
    profileId: String(input?.profileId || "").trim() || `profile-${Date.now()}`,
    profileName: String(input?.profileName || "").trim() || "Unnamed",
    serverBaseURL: String(input?.serverBaseURL || "").trim().replace(/\/+$/, ""),
    releaseToken: String(input?.releaseToken || "").trim(),
    rememberToken: Boolean(input?.rememberToken),
    timeoutMs: Math.max(1000, Number(input?.timeoutMs || 10000))
  }
}

function getActiveProfile(config) {
  return config.profiles.find((profile) => profile.profileId === config.activeProfileId) || null
}

function buildWindow() {
  const window = new BrowserWindow({
    width: 1440,
    height: 920,
    minWidth: 1180,
    minHeight: 760,
    backgroundColor: "#f5f7fa",
    webPreferences: {
      preload: path.join(__dirname, "../preload/index.cjs"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false,
    }
  })
  window.webContents.on("did-fail-load", (_, errorCode, errorDescription, validatedURL) => {
    window.loadURL(`data:text/html,${encodeURIComponent(`
      <!doctype html>
      <html>
        <body style="font-family: Helvetica, Arial, sans-serif; padding: 32px; background: #f5f7fa; color: #303133;">
          <h2>Admin Client load failed</h2>
          <p><strong>URL:</strong> ${validatedURL || "(unknown)"}</p>
          <p><strong>Error:</strong> ${errorCode} ${errorDescription}</p>
          <p>Dev mode: start <code>npm run dev:renderer</code> first, then <code>npm run dev:app</code>.</p>
          <p>Prod mode: run <code>npm run build</code> first, then <code>npm run start</code>.</p>
        </body>
      </html>
    `)}`)
  })
  const devUrl = process.env.ADMIN_RENDERER_DEV_URL
  if (devUrl) {
    window.loadURL(devUrl)
    window.webContents.openDevTools({ mode: "detach" })
    return
  }
  const prodIndex = path.resolve(__dirname, "../renderer/dist/index.html")
  if (!fs.existsSync(prodIndex)) {
    throw new Error("admin renderer dist not found, run `npm run build` in admin-client/")
  }
  window.loadFile(prodIndex)
}

function recordOperation(entry) {
  recentOperations.unshift({
    id: `op-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    at: new Date().toISOString(),
    ...entry
  })
  if (recentOperations.length > maxRecentOperations) {
    recentOperations.length = maxRecentOperations
  }
}

function resolveAdminURL(profile, moduleName, resourcePath, query) {
  const prefix = adminModulePrefixes[moduleName]
  if (!prefix) {
    throw new Error(`unsupported admin module: ${moduleName}`)
  }
  const normalizedPath = String(resourcePath || "").trim()
  if (normalizedPath.includes("..")) {
    throw new Error("invalid admin path")
  }
  const pathname = `${prefix}${normalizedPath.startsWith("/") ? normalizedPath : normalizedPath ? `/${normalizedPath}` : ""}`
  if (!pathname.startsWith("/api/admin/")) {
    throw new Error("admin path must stay under /api/admin/")
  }
  const url = new URL(profile.serverBaseURL + pathname)
  if (query && typeof query === "object") {
    for (const [key, value] of Object.entries(query)) {
      if (value === undefined || value === null || value === "") continue
      url.searchParams.set(key, String(value))
    }
  }
  return url
}

async function requestAdmin(profile, method, moduleName, resourcePath, payload, query) {
  const url = resolveAdminURL(profile, moduleName, resourcePath, query)
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), profile.timeoutMs || 10000)
  try {
    const response = await fetch(url, {
      method,
      headers: {
        "Content-Type": "application/json",
        "X-Release-Token": profile.releaseToken
      },
      body: method === "POST" && payload !== undefined ? JSON.stringify(payload) : undefined,
      signal: controller.signal
    })
    const rawText = await response.text()
    let parsed = null
    try {
      parsed = rawText ? JSON.parse(rawText) : null
    } catch {
      parsed = null
    }
    if (!response.ok) {
      const message =
        parsed?.error?.message ||
        parsed?.message ||
        rawText ||
        `${response.status} ${response.statusText}`
      throw new Error(message)
    }
    return parsed?.data ?? parsed
  } finally {
    clearTimeout(timeout)
  }
}

app.whenReady().then(() => {
  buildWindow()

  ipcMain.handle("admin:getProfiles", async () => {
    const config = loadConfig()
    return config
  })

  ipcMain.handle("admin:saveProfile", async (_, profileInput) => {
    const config = loadConfig()
    const profile = normalizeProfile(profileInput)
    const existingIndex = config.profiles.findIndex((item) => item.profileId === profile.profileId)
    if (existingIndex >= 0) {
      config.profiles[existingIndex] = profile
    } else {
      config.profiles.push(profile)
    }
    if (!config.activeProfileId) {
      config.activeProfileId = profile.profileId
    }
    saveConfig(config)
    return config
  })

  ipcMain.handle("admin:deleteProfile", async (_, profileId) => {
    const config = loadConfig()
    config.profiles = config.profiles.filter((item) => item.profileId !== profileId)
    if (config.activeProfileId === profileId) {
      config.activeProfileId = config.profiles[0]?.profileId || ""
    }
    saveConfig(config)
    return config
  })

  ipcMain.handle("admin:setActiveProfile", async (_, profileId) => {
    const config = loadConfig()
    config.activeProfileId = String(profileId || "")
    saveConfig(config)
    return getActiveProfile(config)
  })

  ipcMain.handle("admin:getActiveProfile", async () => {
    const config = loadConfig()
    return getActiveProfile(config)
  })

  ipcMain.handle("admin:testConnection", async (_, profileId) => {
    const config = loadConfig()
    const profile = config.profiles.find((item) => item.profileId === profileId) || getActiveProfile(config)
    if (!profile) {
      throw new Error("no active profile configured")
    }
    const data = await requestAdmin(profile, "GET", "users", "/ban-reasons", undefined, undefined)
    return {
      ok: true,
      profileId: profile.profileId,
      profileName: profile.profileName,
      result: data
    }
  })

  ipcMain.handle("admin:apiGet", async (_, moduleName, resourcePath, query) => {
    const config = loadConfig()
    const profile = getActiveProfile(config)
    if (!profile) {
      throw new Error("no active profile configured")
    }
    const result = await requestAdmin(profile, "GET", moduleName, resourcePath, undefined, query)
    recordOperation({
      profileName: profile.profileName,
      moduleName,
      method: "GET",
      resourcePath
    })
    return result
  })

  ipcMain.handle("admin:apiPost", async (_, moduleName, resourcePath, payload) => {
    const config = loadConfig()
    const profile = getActiveProfile(config)
    if (!profile) {
      throw new Error("no active profile configured")
    }
    const result = await requestAdmin(profile, "POST", moduleName, resourcePath, payload, undefined)
    recordOperation({
      profileName: profile.profileName,
      moduleName,
      method: "POST",
      resourcePath
    })
    return result
  })

  ipcMain.handle("admin:getRecentOperations", async () => recentOperations)

  ipcMain.handle("admin:readGameData", async (_, tableName) => {
    const allowedTables = new Set(["title_prefixes", "title_suffixes", "avatars", "avatar_frames"])
    if (!allowedTables.has(tableName)) {
      throw new Error(`unsupported game-data table: ${tableName}`)
    }
    const filePath = path.join(__dirname, "..", "..", "packages", "game-data", "source", "json", `${tableName}.json`)
    try {
      const raw = fs.readFileSync(filePath, "utf-8")
      const parsed = JSON.parse(raw)
      return parsed[tableName] || []
    } catch (err) {
      return []
    }
  })
})
