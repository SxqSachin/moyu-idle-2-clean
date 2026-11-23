import { createApp, defineComponent, h } from "vue"
import { createPinia } from "pinia"
import i18n, { setAppLocale } from "@/i18n"
import "@/style.css"
import ItemPopoverContent from "@/components/game/item/ItemPopoverContent.vue"
import { loadAllGameData } from "@/services/game-data-runtime"

await setAppLocale("zh-CN")
await loadAllGameData()

const Host = defineComponent({
  setup() {
    return () => h("main", { class: "sachet-harness" }, [
      h("header", { class: "sachet-harness__header" }, [
        h("h1", "诅咒香囊"),
        h("p", "玩家实际看到的装备说明")
      ]),
      h("section", { class: "sachet-harness__grid" }, [
        h("article", { class: "sachet-harness__item" }, [
          h(ItemPopoverContent, { itemId: "cursed_silk_sachet", active: false })
        ])
      ])
    ])
  }
})

createApp(Host).use(createPinia()).use(i18n).mount("#app")

const style = document.createElement("style")
style.textContent = `
  body {
    margin: 0;
    background: #f4f6f8;
    color: var(--app-text);
  }
  .sachet-harness {
    width: min(760px, calc(100% - 24px));
    margin: 0 auto;
    padding: 28px 0 56px;
  }
  .sachet-harness__header {
    margin-bottom: 18px;
    padding: 0 4px;
  }
  .sachet-harness__header h1 {
    margin: 0;
    font-size: 24px;
    line-height: 1.3;
    letter-spacing: 0;
  }
  .sachet-harness__header p {
    margin: 5px 0 0;
    color: var(--app-muted);
    font-size: 13px;
  }
  .sachet-harness__grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    align-items: start;
    gap: 14px;
  }
  .sachet-harness__item {
    min-width: 0;
    overflow: hidden;
    border: 1px solid var(--app-border);
    border-radius: 6px;
    background: var(--app-surface);
    box-shadow: 0 8px 22px rgb(18 28 45 / 7%);
    padding: 18px;
  }
  @media (max-width: 720px) {
    .sachet-harness {
      width: calc(100% - 16px);
      padding-top: 16px;
    }
    .sachet-harness__header h1 {
      font-size: 20px;
    }
    .sachet-harness__grid {
      grid-template-columns: minmax(0, 1fr);
      gap: 10px;
    }
    .sachet-harness__item {
      padding: 12px;
    }
  }
`
document.head.appendChild(style)
