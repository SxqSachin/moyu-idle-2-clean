import { createApp } from "vue"
import { createPinia } from "pinia"
import App from "@/App.vue"
import router from "@/router"
import i18n, { ensureLocaleNamespaces, getLocaleNamespacesForPath, normalizeSupportedLocale, syncDocumentTitle } from "@/i18n"
import { useSkin } from "@/composables/useSkin"

// import * as ElementPlusIconsVue from '@element-plus/icons-vue'

async function bootstrap() {
  const isGameRoute = window.location.pathname.startsWith("/game")
  const hasToken = !!localStorage.getItem("token")

  if (isGameRoute && hasToken) {
    void import("@services/game-data-runtime")
      .then(({ loadAllGameData }) => loadAllGameData())
      .catch(() => undefined)
  }

  const initialLocale = normalizeSupportedLocale(i18n.global.locale.value)
  await ensureLocaleNamespaces(initialLocale, getLocaleNamespacesForPath(window.location.pathname))
  syncDocumentTitle()

  useSkin().init()

  const app = createApp(App).use(createPinia()).use(router).use(i18n)

  // for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  //   app.component(key, component)
  // }

  app.mount("#app")

  void router.isReady().then(() => {
    syncDocumentTitle()
    window.requestAnimationFrame(() => {
      document.getElementById("app-preboot")?.remove()
    })
  })
}

void bootstrap()
