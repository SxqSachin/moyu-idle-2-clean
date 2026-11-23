import { updateLogMenuRoutes, type UpdateLogMenuKey } from "@/data/update-log"
import { getAttributeDefinition } from "@services/attribute-display"
import { getActionDefinition } from "@services/action-definition"
import { getEnemyDefinition } from "@services/enemy-definition"
import { getAffixDefinition, getItemDefinition } from "@services/item-definition"
import { getSkillDefinition } from "@services/skill-definition"

type UpdateLogQuickRefKind = "skill" | "item" | "affix" | "enemy" | "attr" | "action"

export type UpdateLogSegment =
  | {
      kind: "text"
      text: string
    }
  | {
      kind: "menu"
      key: UpdateLogMenuKey
      label: string
      to: string
    }
  | {
      kind: UpdateLogQuickRefKind
      id: string
    }

const TOKEN_PATTERN = /\[([^\]\r\n]+)\]\(menu:([A-Za-z][A-Za-z0-9_-]*)\)|(skill|item|affix|enemy|attr|action):([A-Za-z0-9_](?:[A-Za-z0-9_.-]*[A-Za-z0-9_])?)/g
const TOKEN_BOUNDARY_PATTERN = /[\s.,!?;:()[\]{}<>"'`~@#$%^&*+=|\\/，。！？；：、（）【】《》]/

function isTokenBoundary(char: string): boolean {
  if (!char) return true
  return TOKEN_BOUNDARY_PATTERN.test(char)
}

function isUpdateLogMenuKey(value: string): value is UpdateLogMenuKey {
  return Object.prototype.hasOwnProperty.call(updateLogMenuRoutes, value)
}

function quickReferenceExists(kind: UpdateLogQuickRefKind, id: string): boolean {
  switch (kind) {
    case "skill":
      return !!getSkillDefinition(id)
    case "item":
      return !!getItemDefinition(id)
    case "affix":
      return !!getAffixDefinition(id)
    case "enemy":
      return !!getEnemyDefinition(id)
    case "attr":
      return !!getAttributeDefinition(id)
    case "action":
      return !!getActionDefinition(id)
  }
}

export function parseUpdateLogLineSegments(line: string): UpdateLogSegment[] {
  const normalizedLine = String(line || "")
  const segments: UpdateLogSegment[] = []
  let cursor = 0
  TOKEN_PATTERN.lastIndex = 0
  let match = TOKEN_PATTERN.exec(normalizedLine)

  while (match) {
    const matchStart = match.index
    const fullToken = match[0]
    const matchEnd = matchStart + fullToken.length
    const menuLabel = String(match[1] || "").trim()
    const menuKey = String(match[2] || "")
    const quickRefKind = match[3] as UpdateLogQuickRefKind | undefined
    const quickRefID = String(match[4] || "")

    const validMenuLink = menuLabel !== "" && isUpdateLogMenuKey(menuKey)
    const validQuickReference = !!quickRefKind
      && isTokenBoundary(matchStart > 0 ? normalizedLine[matchStart - 1] : "")
      && isTokenBoundary(matchEnd < normalizedLine.length ? normalizedLine[matchEnd] : "")
      && quickReferenceExists(quickRefKind, quickRefID)

    if (validMenuLink || validQuickReference) {
      if (matchStart > cursor) {
        segments.push({ kind: "text", text: normalizedLine.slice(cursor, matchStart) })
      }
      if (validMenuLink) {
        segments.push({
          kind: "menu",
          key: menuKey,
          label: menuLabel,
          to: updateLogMenuRoutes[menuKey],
        })
      } else if (quickRefKind) {
        segments.push({ kind: quickRefKind, id: quickRefID })
      }
      cursor = matchEnd
    }
    match = TOKEN_PATTERN.exec(normalizedLine)
  }

  if (cursor < normalizedLine.length) {
    segments.push({ kind: "text", text: normalizedLine.slice(cursor) })
  }
  if (segments.length <= 0) {
    return [{ kind: "text", text: normalizedLine }]
  }
  return segments
}
