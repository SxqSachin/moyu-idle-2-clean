import { createApp, defineComponent, h } from "vue"
import { createPinia } from "pinia"
import i18n, { setAppLocale } from "@/i18n"
import "@/style.css"
import ActionPanel from "@/components/game/action/ActionPanel.vue"
import { loadAllGameData } from "@/services/game-data-runtime"

await setAppLocale("zh-CN")
await loadAllGameData()

const Host = defineComponent({
  setup() {
    return () => h("main", { class: "crafting-harness" }, [
      h(ActionPanel, { menu: "crafting", titleKey: "game.menu.crafting" })
    ])
  }
})

createApp(Host).use(createPinia()).use(i18n).mount("#app")

const style = document.createElement("style")
style.textContent = `
  body { margin: 0; background: var(--app-bg); }
  .crafting-harness { min-height: 100vh; }
`
document.head.appendChild(style)
