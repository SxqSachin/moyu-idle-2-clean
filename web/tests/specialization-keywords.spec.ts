import { describe, expect, it } from "vitest"
import {
  buildSpecializationKeywordTextParts,
  normalizeSpecializationSummaryLine,
  resolveSpecializationKeywords
} from "../src/services/specialization-keywords"
import type { SpecializationNode, SpecializationTree } from "../src/services/specialization"

function node(id: string, overrides: Partial<SpecializationNode> = {}): SpecializationNode {
  return {
    id,
    kind: "technique",
    schoolIds: ["intelligence"],
    nameKey: `${id}.name`,
    descKey: `${id}.desc`,
    requirements: { all: [], anyOf: [] },
    levels: [{ level: 1, cost: 2, grants: [] }],
    ui: {},
    ...overrides
  }
}

function tree(nodes: SpecializationNode[]): SpecializationTree {
  return {
    id: "test",
    schemaVersion: 2,
    rulesetVersion: "test-v1",
    balanceRevision: 1,
    maxActiveRootNodes: 3,
    nameKey: "test.name",
    descKey: "test.desc",
    schools: [{ id: "intelligence", nameKey: "int.name", descKey: "int.desc", attributeKey: "intelligence" }],
    nodes,
    keywords: [{
      id: "echo_cast",
      mechanicKind: "mirror_cast_skill",
      runtimeIds: ["echo_hook"],
      sourceNodeIds: ["echo_source"],
      nameKey: "keyword.echo.name",
      summaryKey: "keyword.echo.summary",
      mechanicDetailsKey: "keyword.echo.details",
      sourceHintKey: "keyword.echo.source"
    }]
  }
}

const messages: Record<string, string> = {
  "game.specialization.keyword.sourceSeparator": "、",
  "game.specialization.keyword.sourceNodes": "由{nodes}提供",
  "game.specialization.keyword.sourceNodesMore": "由{nodes}等 {count} 个节点提供",
  "keyword.echo.name": "复诵",
  "keyword.echo.summary": "再次施放刚才的技能。",
  "keyword.echo.details": "复诵不会触发另一轮复诵。",
  "keyword.echo.source": "点出回声节点后获得。"
}

const t = (key: string, params?: Record<string, unknown>) => {
  let value = messages[key] || key
  for (const [param, replacement] of Object.entries(params || {})) value = value.replace(`{${param}}`, String(replacement))
  return value
}

describe("specialization keyword projection", () => {
  it("prefers stable node keyword ids and canonical keyword copy", () => {
    const source = node("echo_source", { keywordIds: ["echo_cast"] })
    const resolved = resolveSpecializationKeywords(tree([source]), source, {
      locale: "zh-CN",
      t,
      nodeName: () => "再来一遍"
    })

    expect(resolved).toEqual([expect.objectContaining({
      id: "echo_cast",
      name: "复诵",
      description: "再次施放刚才的技能。",
      trigger: "复诵不会触发另一轮复诵。",
      source: "点出回声节点后获得。"
    })])
  })

  it("supports localized v5 keyword fields without reading Chinese effect copy", () => {
    const source = node("echo_source", { keywordIds: ["echo_cast"] })
    const resolved = resolveSpecializationKeywords(tree([source]), source, {
      locale: "zh-CN",
      localizedKeywords: {
        echo_cast: {
          name: "回声复诵",
          playerSummary: "重新结算一次技能。",
          mechanicDetails: "不会递归触发。",
          sourceHint: "由回声施法路线提供。"
        }
      },
      t,
      nodeName: () => "再来一遍"
    })

    expect(resolved[0]).toMatchObject({
      name: "回声复诵",
      description: "重新结算一次技能。",
      trigger: "不会递归触发。",
      source: "由回声施法路线提供。"
    })
  })

  it("does not infer compatibility keywords from untagged DSL", () => {
    const untagged = node("untagged_intercept", {
      schoolIds: ["stamina"],
      levels: [{
        level: 1,
        cost: 2,
        grants: [{
          id: "untagged_intercept",
          kind: "battle_hook",
          battleHooks: [{ id: "guard", effects: [{ kind: "redirect_damage", targetMode: "self", ratio: 0.3 }] }]
        }]
      }]
    })
    const resolved = resolveSpecializationKeywords(tree([untagged]), untagged, {
      locale: "zh-CN",
      t,
      nodeName: () => "并肩承伤"
    })

    expect(resolved).toEqual([])
  })

  it("keeps the debuff family and concrete branch buffs independently resolvable", () => {
    const source = node("mark_source", {
      keywordIds: ["specialization_mark", "sentinel_mark"],
      levels: [{
        level: 1,
        cost: 2,
        grants: [{
          id: "mark",
          kind: "battle_hook",
          battleHooks: [{ effects: [{ kind: "mark_target", markId: "specialization_sentinel_target" }] }]
        }]
      }]
    })
    const currentTree = tree([source])
    currentTree.keywords = [
      { id: "specialization_mark", mechanicKind: "debuff_family", runtimeIds: ["target_has_any_buff"] },
      { id: "sentinel_mark", mechanicKind: "debuff", runtimeIds: ["specialization_sentinel_target"] },
      { id: "observed_gap", mechanicKind: "debuff", runtimeIds: ["perception_observed_gap"] }
    ]

    const resolved = resolveSpecializationKeywords(currentTree, source, {
      locale: "zh-CN",
      localizedKeywords: {
        specialization_mark: { name: "专精标记", playerSummary: "泛型。" },
        sentinel_mark: { name: "哨卫标记", playerSummary: "具体标记。" },
        observed_gap: { name: "观察", playerSummary: "另一分支减益。" }
      },
      t,
      nodeName: () => "互相照看"
    })

    expect(new Set(resolved.map((entry) => entry.id))).toEqual(new Set(["specialization_mark", "sentinel_mark", "observed_gap"]))
    expect(resolved.find((entry) => entry.id === "specialization_mark")?.buffIds)
      .toEqual(["specialization_sentinel_target", "perception_observed_gap"])
    expect(resolved.find((entry) => entry.id === "sentinel_mark")?.buffIds)
      .toEqual(["specialization_sentinel_target"])
  })

  it("renders explicit keyword tokens and plain-copy aliases as tag parts", () => {
    const keyword = {
      id: "echo_cast",
      name: "复诵",
      description: "再次施放。",
      trigger: "",
      source: "",
      sourceNodeIds: [],
      runtimeIds: ["mirror_cast_skill"],
      buffIds: [],
      aliases: ["复诵"]
    }
    const explicit = buildSpecializationKeywordTextParts("获得[[keyword:echo_cast]]。", [keyword])
    const plain = buildSpecializationKeywordTextParts("下一次复诵提高。", [keyword])

    expect(explicit.find((part) => part.kind === "keyword")).toMatchObject({ text: "复诵" })
    expect(plain.find((part) => part.kind === "keyword")).toMatchObject({ text: "复诵" })
    expect(normalizeSpecializationSummaryLine({ tokens: ["获得", { keywordId: "echo_cast" }, "。"] }))
      .toBe("获得[[keyword:echo_cast]]。")
  })
})
