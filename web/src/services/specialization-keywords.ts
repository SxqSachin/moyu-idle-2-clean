import type {
  SpecializationEffectSummaryLine,
  SpecializationKeywordCollection,
  SpecializationKeywordDefinition,
  SpecializationLocalizedKeyword,
  SpecializationNode,
  SpecializationTree
} from "@services/specialization"
import {
  buildGameKeywordTextParts,
  type GameKeywordDisplay,
  type GameKeywordTextPart
} from "@services/game-keyword"

export type ResolvedSpecializationKeyword = GameKeywordDisplay & {
  trigger: string
  source: string
  sourceNodeIds: string[]
  aliases: string[]
  mechanicKind?: string
  runtimeIds: string[]
  buffIds: string[]
}

export type SpecializationKeywordTextPart = GameKeywordTextPart<ResolvedSpecializationKeyword>

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

export type ResolveSpecializationKeywordOptions = {
  locale: string
  localizedKeywords?: Record<string, SpecializationLocalizedKeyword>
  t: TranslateFn
  nodeName: (nodeId: string) => string
}

export function normalizeSpecializationSummaryLine(line: SpecializationEffectSummaryLine): string {
  if (typeof line === "string") return line
  if (Array.isArray(line.tokens)) {
    return line.tokens.map((token) => {
      if (typeof token === "string") return token
      const keywordId = String(token.keywordId || "").trim()
      if (keywordId) return `[[keyword:${keywordId}]]`
      return String(token.text || "")
    }).join("")
  }
  return String(line.text || "")
}

export function resolveSpecializationKeywords(
  tree: SpecializationTree,
  node: SpecializationNode,
  options: ResolveSpecializationKeywordOptions
): ResolvedSpecializationKeyword[] {
  const definitions = new Map<string, SpecializationKeywordDefinition>()
  for (const definition of normalizeKeywordCollection(tree.keywords)) definitions.set(definition.id, definition)

  const explicitIds = new Set<string>([
    ...(node.keywordIds || []),
    ...node.levels.flatMap((level) => level.keywordIds || []),
    ...extractSummaryKeywordIds(tree, node.id, options.locale)
  ].map((id) => String(id || "").trim()).filter(Boolean))

  const genericDebuffIds = new Set(Array.from(explicitIds).filter((id) => {
    const mechanicKind = definitions.get(id)?.mechanicKind
    return id === "recognized_debuffs" || id === "specialization_mark"
      || mechanicKind === "mark_family" || mechanicKind === "debuff_family"
  }))
  const specificDebuffDefinitions = Array.from(definitions.values()).filter((definition) => {
    const mechanicKind = String(definition.mechanicKind || "").trim().toLowerCase()
    return mechanicKind === "mark" || mechanicKind === "debuff"
  })
  if (genericDebuffIds.size > 0) specificDebuffDefinitions.forEach((definition) => explicitIds.add(definition.id))
  const recognizedBuffIds = Array.from(new Set(specificDebuffDefinitions.flatMap((definition) => definition.runtimeIds || [])))

  const resolved: ResolvedSpecializationKeyword[] = []
  for (const id of explicitIds) {
    const definition = definitions.get(id) || { id }
    const mechanicKind = String(definition.mechanicKind || "").trim().toLowerCase()
    const localized = options.localizedKeywords?.[id] || {}
    const name = firstText(
      localized.name,
      localized.label,
      readLooseText(definition, "name"),
      readLooseText(definition, "label"),
      translateKey(definition.nameKey, options.t),
      options.t(`game.specialization.keyword.${id}.name`),
      id
    )
    const sourceNodeIds = resolveKeywordSourceNodeIds(node, definition)
    const sourceNames = sourceNodeIds.map(options.nodeName).filter(Boolean)
    const sourceNodeSummary = sourceNames.length > 3
      ? options.t("game.specialization.keyword.sourceNodesMore", {
          nodes: sourceNames.slice(0, 3).join(options.t("game.specialization.keyword.sourceSeparator")),
          count: sourceNames.length
        })
      : sourceNames.length > 0
        ? options.t("game.specialization.keyword.sourceNodes", { nodes: sourceNames.join(options.t("game.specialization.keyword.sourceSeparator")) })
        : ""
    const description = firstText(
      localized.description,
      localized.effect,
      readLooseText(definition, "description"),
      readLooseText(definition, "effect"),
      localized.playerSummary,
      translateKey(definition.summaryKey, options.t),
      translateKey(definition.descriptionKey, options.t),
      options.t(`game.specialization.keyword.${id}.description`),
      name
    )
    const trigger = firstText(
      localized.trigger,
      localized.mechanicDetails,
      readLooseText(definition, "trigger"),
      translateKey(definition.mechanicDetailsKey, options.t),
      translateKey(definition.triggerKey, options.t),
      options.t(`game.specialization.keyword.${id}.trigger`)
    )
    const source = firstText(
      localized.source,
      localized.acquisition,
      localized.sourceHint,
      readLooseText(definition, "source"),
      readLooseText(definition, "acquisition"),
      translateKey(definition.sourceKey, options.t),
      translateKey(definition.sourceHintKey, options.t),
      sourceNodeSummary
    )
    const buffPhraseAliases = mechanicKind === "mark" || mechanicKind === "debuff"
      ? options.locale.toLowerCase().startsWith("zh")
        ? [`${name}减益`]
        : [`${name} debuff`]
      : []
    resolved.push({
      id,
      name,
      description,
      trigger,
      source,
      sourceNodeIds,
      mechanicKind: definition.mechanicKind,
      runtimeIds: [...(definition.runtimeIds || [])],
      buffIds: mechanicKind === "mark_family" || mechanicKind === "debuff_family"
        ? recognizedBuffIds
        : mechanicKind === "mark" || mechanicKind === "debuff" || mechanicKind === "buff"
          ? [...(definition.runtimeIds || [])]
          : [],
      aliases: Array.from(new Set([
        name,
        ...buffPhraseAliases,
        ...(localized.aliases || []),
        ...(definition.aliases || []),
      ].map((alias) => String(alias || "").trim()).filter(Boolean)))
    })
  }
  return resolved.sort((left, right) => left.name.localeCompare(right.name, options.locale))
}

export function buildSpecializationKeywordTextParts(
  text: string,
  keywords: readonly ResolvedSpecializationKeyword[]
): SpecializationKeywordTextPart[] {
  return buildGameKeywordTextParts(text, keywords)
}

function normalizeKeywordCollection(collection?: SpecializationKeywordCollection): SpecializationKeywordDefinition[] {
  if (Array.isArray(collection)) {
    return collection.filter((entry) => entry && typeof entry === "object" && String(entry.id || "").trim())
      .map((entry) => ({ ...entry, id: String(entry.id).trim() }))
  }
  if (!collection || typeof collection !== "object") return []
  return Object.entries(collection).flatMap(([id, raw]) => {
    if (!raw || typeof raw !== "object") return []
    return [{ ...raw, id: String(raw.id || id).trim() }]
  }).filter((entry) => entry.id)
}

function extractSummaryKeywordIds(tree: SpecializationTree, nodeId: string, locale: string): string[] {
  const source = tree.i18n?.[locale]
    || tree.i18n?.[locale.toLowerCase().startsWith("zh") ? "zh-CN" : "en-US"]
    || tree.i18n?.["zh-CN"]
    || tree.i18n?.["en-US"]
  const localizedNode = source?.nodes?.[nodeId]
  const lines = [
    ...(localizedNode?.effectSummary || []),
    ...(localizedNode?.levelEffectSummary || []).flat()
  ]
  const ids: string[] = []
  for (const line of lines) {
    if (typeof line !== "string" && Array.isArray(line.tokens)) {
      for (const token of line.tokens) {
        if (typeof token !== "string" && token.keywordId) ids.push(String(token.keywordId))
      }
    }
    const text = normalizeSpecializationSummaryLine(line)
    for (const match of text.matchAll(/(?:\[\[|\{\{)keyword:([a-z0-9_.:-]+)(?:\]\]|\}\})/gi)) {
      if (match[1]) ids.push(match[1])
    }
  }
  return ids
}

function resolveKeywordSourceNodeIds(
  currentNode: SpecializationNode,
  definition: SpecializationKeywordDefinition
): string[] {
  const looseDefinition = definition as SpecializationKeywordDefinition & Record<string, unknown>
  const configured = [
    ...(definition.sourceNodeIds || []),
    ...readLooseStringList(looseDefinition.sourceNodes),
    ...readLooseStringList(looseDefinition.grantedByNodeIds)
  ]
  if (configured.length > 0) return Array.from(new Set(configured))
  return [currentNode.id]
}

function translateKey(key: string | undefined, t: TranslateFn): string {
  if (!key) return ""
  const value = t(key)
  return value === key ? "" : value
}

function firstText(...values: unknown[]): string {
  for (const value of values) {
    const text = String(value || "").trim()
    if (text && !text.startsWith("game.specialization.keyword.")) return text
  }
  return ""
}

function readLooseText(value: unknown, key: string): string {
  if (!value || typeof value !== "object") return ""
  const raw = (value as Record<string, unknown>)[key]
  return typeof raw === "string" ? raw : ""
}

function readLooseStringList(value: unknown): string[] {
  return Array.isArray(value) ? value.map((entry) => String(entry || "").trim()).filter(Boolean) : []
}
