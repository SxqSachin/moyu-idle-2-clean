import { defineConfig } from "vitest/config"
import vue from "@vitejs/plugin-vue"
import { fileURLToPath, URL } from "node:url"

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
      "@components": fileURLToPath(new URL("./src/components", import.meta.url)),
      "@services": fileURLToPath(new URL("./src/services", import.meta.url)),
      "@stores": fileURLToPath(new URL("./src/stores", import.meta.url)),
      "@views": fileURLToPath(new URL("./src/views", import.meta.url)),
      "@utils": fileURLToPath(new URL("./src/utils", import.meta.url)),
      "@composables": fileURLToPath(new URL("./src/composables", import.meta.url)),
      "@game-data": fileURLToPath(new URL("../packages/game-data/generated", import.meta.url)),
      "@shared-config": fileURLToPath(new URL("../packages/game-config/source", import.meta.url))
    }
  },
  test: {
    environment: "jsdom",
    setupFiles: ["tests/setup-game-data.ts"],
    include: ["tests/**/*.spec.ts"]
  }
})
