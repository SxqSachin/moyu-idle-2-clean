import { createApp, defineComponent, h } from "vue"
import { createPinia } from "pinia"
import i18n, { setAppLocale } from "@/i18n"
import "@/style.css"
import ItemPopoverContent from "@/components/game/item/ItemPopoverContent.vue"
import { loadAllGameData } from "@/services/game-data-runtime"

await setAppLocale("zh-CN")
await loadAllGameData()

const itemIDs = [
  "cat_sword",
  "dark_cat_sword",
  "king_of_amusement_park",
  "last_light_of_the_parade",
  "jackpot_biased_token",
  "hollow_heart"
]

const Host = defineComponent({
  setup() {
    return () => h("main", { class: "item-copy-harness" }, [
      h("header", { class: "item-copy-harness__header" }, [
        h("h1", "游乐园珍藏"),
        h("p", "玩家实际看到的名称、说明与战斗效果")
      ]),
      h(
        "section",
        { class: "item-copy-harness__grid" },
        itemIDs.map((itemID) => h(
          "article",
          { class: "item-copy-harness__item", key: itemID },
          [h(ItemPopoverContent, { itemId: itemID, active: false })]
        ))
      )
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
  .item-copy-harness {
    width: min(1180px, calc(100% - 24px));
    margin: 0 auto;
    padding: 28px 0 56px;
  }
  .item-copy-harness__header {
    margin-bottom: 18px;
    padding: 0 4px;
  }
  .item-copy-harness__header h1 {
    margin: 0;
    font-size: 24px;
    line-height: 1.3;
    letter-spacing: 0;
  }
  .item-copy-harness__header p {
    margin: 5px 0 0;
    color: var(--app-muted);
    font-size: 13px;
  }
  .item-copy-harness__grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    align-items: start;
    gap: 14px;
  }
  .item-copy-harness__item {
    min-width: 0;
    overflow: hidden;
    border: 1px solid var(--app-border);
    border-radius: 6px;
    background: var(--app-surface);
    box-shadow: 0 8px 22px rgb(18 28 45 / 7%);
    padding: 18px;
  }
  @media (max-width: 720px) {
    .item-copy-harness {
      width: calc(100% - 16px);
      padding-top: 16px;
    }
    .item-copy-harness__header h1 {
      font-size: 20px;
    }
    .item-copy-harness__grid {
      grid-template-columns: minmax(0, 1fr);
      gap: 10px;
    }
    .item-copy-harness__item {
      padding: 12px;
    }
  }
`
document.head.appendChild(style)
