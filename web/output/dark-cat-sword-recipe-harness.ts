import { createApp, defineComponent, h, ref } from "vue"
import { createPinia } from "pinia"
import i18n, { setAppLocale } from "@/i18n"
import "@/style.css"
import ActionDetailDialog from "@/components/game/action/ActionDetailDialog.vue"
import { listActionDefinitions } from "@/services/action-definition"
import { loadAllGameData } from "@/services/game-data-runtime"

await setAppLocale("zh-CN")
await loadAllGameData()

const action = listActionDefinitions().find((row) => row.id === "craft_dark_cat_sword") || null

const Host = defineComponent({
  setup() {
    const visible = ref(true)
    return () => h("main", { class: "min-h-screen bg-[var(--app-bg)]" }, [
      h(ActionDetailDialog, {
        modelValue: visible.value,
        "onUpdate:modelValue": (next: boolean) => { visible.value = next },
        action,
        count: 1,
        currentAttributes: { crafting: 100 }
      })
    ])
  }
})

createApp(Host).use(createPinia()).use(i18n).mount("#app")
