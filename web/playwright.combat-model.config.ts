import { defineConfig } from "@playwright/test"

export default defineConfig({
  testDir: "./e2e",
  testMatch: "combat-model-specialization.spec.ts",
  timeout: 30_000,
  use: {
    baseURL: "http://127.0.0.1:4178",
    channel: "chrome",
    headless: true
  },
  webServer: {
    command: "npm --prefix ../combat-model-vue3 run dev -- --host 127.0.0.1 --port 4178",
    url: "http://127.0.0.1:4178",
    reuseExistingServer: true,
    timeout: 120_000
  }
})
