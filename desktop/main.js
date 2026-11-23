const { app, BrowserWindow } = require("electron")
const path = require("path")
const fs = require("fs")

function createWindow() {
  const win = new BrowserWindow({
    width: 1366,
    height: 840,
    backgroundColor: "#0a0a0a",
    webPreferences: {
      preload: path.join(__dirname, "preload.js")
    }
  })
  const devUrl = process.env.WEB_DEV_URL
  if (devUrl) {
    win.loadURL(devUrl)
    return
  }

  const prodIndex = path.resolve(__dirname, "../web/dist/index.html")
  if (!fs.existsSync(prodIndex)) {
    throw new Error("web dist not found, run `npm run build:web` in desktop/")
  }
  win.loadFile(prodIndex)
}

app.whenReady().then(createWindow)
