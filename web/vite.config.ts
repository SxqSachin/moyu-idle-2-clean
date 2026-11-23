import { defineConfig } from "vite"
import vue from "@vitejs/plugin-vue"
import Components from "unplugin-vue-components/vite"
import { ElementPlusResolver } from "unplugin-vue-components/resolvers"
import { fileURLToPath, URL } from "node:url"

const matchesPackage = (id: string, pkg: string) =>
  id.includes(`/node_modules/${pkg}/`) || id.includes(`/node_modules/.pnpm/${pkg.replace("/", "+")}@`)

const normalizeModuleId = (id: string) => id.split("?")[0].replace(/\\/g, "/")

const smallGameDataChunkNames = new Set([
  "buff_type",
  "item_types",
  "rarity_definitions",
  "skill_type",
  "buildings",
  "world",
  "weapon_classes",
  "buff",
  "buff_effects"
])

const resolveGameDataChunk = (id: string) => {
  const match = normalizeModuleId(id).match(/packages\/game-data\/generated\/chunks\/(.+)\.json$/)
  if (!match) return undefined
  const chunkName = match[1].replace(/[^a-zA-Z0-9_-]/g, "-")
  if (smallGameDataChunkNames.has(chunkName)) return "game-data-small"
  return `game-data-${chunkName}`
}

const resolveAppChunk = (id: string) => {
  const normalizedId = normalizeModuleId(id)
  if (!normalizedId.includes("/web/src/")) return undefined
  const localeMatch = normalizedId.match(/\/web\/src\/locales\/([^/]+)\//)
  if (localeMatch?.[1]) {
    return `locale-${localeMatch[1].replace(/[^a-zA-Z0-9_-]/g, "-")}`
  }
  if (normalizedId.includes("/web/src/views/game/boot/")) return undefined
  if (normalizedId.includes("/web/src/views/game/")) return "game-ui"
  if (normalizedId.includes("/web/src/components/game/")) return "game-ui"
  if (
    normalizedId.includes("/web/src/components/common/") &&
    !normalizedId.includes("/web/src/components/common/Instruction")
  ) {
    return "game-ui"
  }
  if (normalizedId.includes("/web/src/composables/useSkin.ts")) return undefined
  if (normalizedId.includes("/web/src/services/api.ts")) return "public-runtime"
  if (normalizedId.includes("/web/src/services/compression.ts")) return "public-runtime"
  if (normalizedId.includes("/web/src/services/locale-switch.ts")) return "public-runtime"
  if (normalizedId.includes("/web/src/services/auth.ts")) return "public-runtime"
  if (normalizedId.includes("/web/src/services/auth-recovery.ts")) return "public-runtime"
  if (normalizedId.includes("/web/src/services/risk-device.ts")) return "public-runtime"
  if (normalizedId.includes("/web/src/stores/game.ts")) return "public-runtime"
  if (normalizedId.includes("/web/src/composables/")) return "game-runtime"
  if (normalizedId.includes("/web/src/services/game-data-runtime.ts")) return undefined
  if (normalizedId.includes("/web/src/services/")) return "game-runtime"
  if (normalizedId.includes("/web/src/stores/")) return "game-runtime"
  if (normalizedId.includes("/web/src/utils/")) return "game-runtime"
  if (normalizedId.includes("/web/src/data/")) return "game-runtime"
  if (normalizedId.includes("/web/src/tutorial/")) return "game-ui"
  return undefined
}

const resolveElementPlusChunk = (id: string) => {
  if (matchesPackage(id, "@element-plus/icons-vue")) return "vendor-ui-icons"
  if (!matchesPackage(id, "element-plus")) return undefined

  const componentMatch = id.match(/element-plus\/(?:es|lib)\/components\/([^/]+)/)
  if (componentMatch?.[1]) return "vendor-ui-components"

  return "vendor-ui-core"
}

const resolveVendorChunk = (id: string) => {
  const elementPlusChunk = resolveElementPlusChunk(id)
  if (elementPlusChunk) return elementPlusChunk
  if (
    matchesPackage(id, "vue") ||
    matchesPackage(id, "@vue/compiler-core") ||
    matchesPackage(id, "@vue/compiler-dom") ||
    matchesPackage(id, "@vue/reactivity") ||
    matchesPackage(id, "@vue/runtime-core") ||
    matchesPackage(id, "@vue/runtime-dom") ||
    matchesPackage(id, "@vue/shared") ||
    matchesPackage(id, "vue-router") ||
    matchesPackage(id, "vue-i18n") ||
    matchesPackage(id, "@intlify/core-base") ||
    matchesPackage(id, "@intlify/message-compiler") ||
    matchesPackage(id, "@intlify/shared") ||
    matchesPackage(id, "dayjs") ||
    matchesPackage(id, "pinia") ||
    matchesPackage(id, "@tanstack/vue-query")
  ) {
    return "vendor-framework-medium"
  }
  if (
    matchesPackage(id, "gsap") ||
    matchesPackage(id, "vuedraggable") ||
    matchesPackage(id, "sortablejs") ||
    matchesPackage(id, "driver.js") ||
    matchesPackage(id, "@iconify/vue") ||
    matchesPackage(id, "@iconify-vue/game-icons")
  ) {
    return "vendor-feature-medium"
  }
  if (
    matchesPackage(id, "@vueuse/core") ||
    matchesPackage(id, "@vueuse/shared") ||
    matchesPackage(id, "@floating-ui/core") ||
    matchesPackage(id, "@floating-ui/dom") ||
    matchesPackage(id, "@floating-ui/utils") ||
    matchesPackage(id, "@popperjs/core") ||
    matchesPackage(id, "@ctrl/tinycolor") ||
    matchesPackage(id, "async-validator") ||
    matchesPackage(id, "lodash-es") ||
    matchesPackage(id, "memoize-one") ||
    matchesPackage(id, "normalize-wheel-es")
  ) {
    return "vendor-ui-core"
  }
  if (
    matchesPackage(id, "axios") ||
    matchesPackage(id, "pako") ||
    matchesPackage(id, "reconnecting-websocket") ||
    matchesPackage(id, "zod")
  ) {
    return "vendor-utils-small"
  }
  return "vendor-misc"
}

export default defineConfig({
  plugins: [
    vue(),
    Components({
      dts: false,
      resolvers: [
        ElementPlusResolver({
          importStyle: "css"
        })
      ]
    })
  ],
  build: {
    modulePreload: false,
    rollupOptions: {
      output: {
        onlyExplicitManualChunks: true,
        manualChunks(id) {
          if (id.includes("packages/game-data/generated/game-data.json")) return "game-data-catalog"
          const gameDataChunk = resolveGameDataChunk(id)
          if (gameDataChunk) return gameDataChunk
          const appChunk = resolveAppChunk(id)
          if (appChunk) return appChunk
          if (!id.includes("node_modules")) return
          return resolveVendorChunk(id)
        }
      }
    }
  },
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
  server: {
    proxy: {
      "/api": "http://localhost:8080",
      "/ws": {
        target: "ws://localhost:8080",
        ws: true
      }
    }
  }
})
