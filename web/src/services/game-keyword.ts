export type GameKeywordDisplay = {
  id: string
  name: string
  description: string
  trigger?: string
  source?: string
  aliases?: string[]
}

export type GameKeywordTextPart<TKeyword extends GameKeywordDisplay = GameKeywordDisplay> =
  | { kind: "text"; key: string; text: string }
  | { kind: "keyword"; key: string; text: string; keyword: TKeyword }

/**
 * Splits player copy into plain text and keyword references. Authored marker
 * tokens win; aliases are only a compatibility path for older plain strings.
 */
export function buildGameKeywordTextParts<TKeyword extends GameKeywordDisplay>(
  text: string,
  keywords: readonly TKeyword[]
): GameKeywordTextPart<TKeyword>[] {
  const byId = new Map(keywords.map((keyword) => [keyword.id, keyword]))
  const marker = /(?:\[\[|\{\{)keyword:([a-z0-9_.:-]+)(?:\]\]|\}\})/gi
  const parts: GameKeywordTextPart<TKeyword>[] = []
  let cursor = 0
  let markerIndex = 0
  for (const match of text.matchAll(marker)) {
    const start = match.index || 0
    parts.push(...tokenizePlainText(text.slice(cursor, start), keywords, `plain-${markerIndex}`))
    const keyword = byId.get(String(match[1] || ""))
    if (keyword) {
      parts.push({ kind: "keyword", key: `marker-${markerIndex}`, text: keyword.name, keyword })
    } else {
      parts.push({ kind: "text", key: `marker-${markerIndex}`, text: String(match[0] || "") })
    }
    cursor = start + String(match[0] || "").length
    markerIndex += 1
  }
  parts.push(...tokenizePlainText(text.slice(cursor), keywords, `plain-${markerIndex}`))
  return parts.filter((part) => part.text.length > 0)
}

function tokenizePlainText<TKeyword extends GameKeywordDisplay>(
  text: string,
  keywords: readonly TKeyword[],
  keyPrefix: string
): GameKeywordTextPart<TKeyword>[] {
  if (!text) return []
  const candidates = keywords.flatMap((keyword) => (keyword.aliases || [keyword.name]).map((alias) => ({ alias, keyword })))
    .filter((candidate) => candidate.alias.length > 0)
    .sort((left, right) => right.alias.length - left.alias.length || left.alias.localeCompare(right.alias))
  if (candidates.length === 0) return [{ kind: "text", key: `${keyPrefix}-0`, text }]
  const pattern = new RegExp(candidates.map((candidate) => escapeRegExp(candidate.alias)).join("|"), "gi")
  const parts: GameKeywordTextPart<TKeyword>[] = []
  let cursor = 0
  let index = 0
  for (const match of text.matchAll(pattern)) {
    const start = match.index || 0
    if (start > cursor) parts.push({ kind: "text", key: `${keyPrefix}-${index++}`, text: text.slice(cursor, start) })
    const value = String(match[0] || "")
    const candidate = candidates.find((entry) => entry.alias.toLocaleLowerCase() === value.toLocaleLowerCase())
    if (candidate) parts.push({ kind: "keyword", key: `${keyPrefix}-${index++}`, text: value, keyword: candidate.keyword })
    else parts.push({ kind: "text", key: `${keyPrefix}-${index++}`, text: value })
    cursor = start + value.length
  }
  if (cursor < text.length) parts.push({ kind: "text", key: `${keyPrefix}-${index}`, text: text.slice(cursor) })
  return parts
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
}
