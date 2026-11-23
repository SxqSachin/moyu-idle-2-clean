import zhCN from "@web/locales/game-data/zh-CN"

type ItemLocale = { name?: string }
const itemMap = (zhCN as unknown as { item?: Record<string, ItemLocale> }).item ?? {}

export function getItemName(itemId: string): string {
  return itemMap[itemId]?.name || itemId
}
