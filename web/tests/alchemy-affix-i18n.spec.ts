import { readFileSync } from "node:fs"
import { resolve } from "node:path"
import { describe, expect, it } from "vitest"
import { createI18n } from "vue-i18n"
import enUS from "../src/locales/game/en-US"
import zhCN from "../src/locales/game/zh-CN"

type ItemRow = {
  id: string
  alchemy?: {
    nameAffix?: {
      prefixKey?: string
      suffixKey?: string
    }
  }
}

const sourcePath = resolve(process.cwd(), "../packages/game-data/source/json/items.json")
const source = JSON.parse(readFileSync(sourcePath, "utf8")) as { items: ItemRow[] }

function localizedString(messages: Record<string, unknown>, path: string): string | null {
  let value: unknown = messages
  for (const part of path.split(".")) {
    if (!value || typeof value !== "object" || Array.isArray(value)) return null
    value = (value as Record<string, unknown>)[part]
  }
  return typeof value === "string" && value.trim() !== "" ? value : null
}

function requiredNameAffixKeys(): Map<string, string[]> {
  const ownersByKey = new Map<string, string[]>()
  for (const item of source.items) {
    const affix = item.alchemy?.nameAffix
    for (const key of [affix?.prefixKey, affix?.suffixKey]) {
      if (!key) continue
      const owners = ownersByKey.get(key) || []
      owners.push(item.id)
      ownersByKey.set(key, owners)
    }
  }
  return ownersByKey
}

describe("alchemy name-affix translations", () => {
  const requiredKeys = requiredNameAffixKeys()

  it.each([
    ["zh-CN", zhCN],
    ["en-US", enUS]
  ])("covers every authored item key in %s", (_locale, messages) => {
    const missing = [...requiredKeys.entries()]
      .filter(([key]) => !localizedString(messages, key))
      .map(([key, itemIDs]) => `${key} (${itemIDs.join(", ")})`)

    expect(missing).toEqual([])
  })

  it("renders the hunter-claw affix without exposing an i18n key", () => {
    const i18n = createI18n({
      legacy: false,
      locale: "zh-CN",
      messages: { "zh-CN": zhCN }
    })
    const t = i18n.global.t
    const name = t("alchemy.affix.template.single", {
      prefix: t("alchemy.affix.prefix.hunter"),
      suffix: t("alchemy.affix.suffix.claw")
    })

    expect(name).toBe("猎手利爪")
  })
})
