import { afterEach, describe, expect, it, vi } from "vitest"
import { createBlankSkillForgeGraph, createOfficialSkillForgeGraph, forgeSkillDraft, normalizeSkillForgeMaterialQuantity } from "../src/services/skill-forge"
import { getLocalizedRuntimeSkillName } from "../src/services/skill-definition"

afterEach(() => {
  vi.unstubAllGlobals()
})

describe("skill forge graph contract", () => {
	it("builds a readable forged skill name instead of exposing the custom id", () => {
		const translations: Record<string, string> = {
			"item.test_material.name": "霜骨",
			"skillForge.name.mighty": "凛冽",
			"skillForge.name.damage": "伤害"
		}
		const t = (key: string) => translations[key] || key
		const name = getLocalizedRuntimeSkillName(t, "custom:93a8", {
			id: "custom:93a8",
			name: "炼成技能",
			nameParams: {
				materialKey: "item.test_material.name",
				effectKey: "skillForge.name.damage",
				traitKey: "skillForge.name.mighty"
			}
		})
		expect(name).toBe("霜骨凛冽")
		expect(name).not.toContain("custom:")
	})

  it("creates a versioned blank graph independent of Vue Flow", () => {
    const graph = createBlankSkillForgeGraph()
    expect(graph.version).toBe(1)
    expect(graph.source).toEqual({ kind: "blank" })
    expect(graph.nodes.filter((node) => node.kind === "effect")).toHaveLength(1)
		expect(graph.nodes.find((node) => node.kind === "effect")?.effectKind).toBe("empty")
    expect(graph.nodes.filter((node) => node.kind === "parameter").map((node) => node.parameterKind)).toEqual(expect.arrayContaining([
      "requirements", "hp_cost", "mp_cost", "cooldown", "conditions", "target_scope", "target_positions", "target_direction",
      "target_count", "target_count_max", "select_rule", "formula", "hit_count", "status_apply", "status_duration"
    ]))
    expect(graph.edges.some((edge) => edge.source === "source" && edge.target === "skill-requirements")).toBe(true)
    expect(graph.edges.some((edge) => edge.source === "effect-1-status-duration" && edge.target === "output")).toBe(true)
    expect(graph.nodes.filter((node) => node.kind === "parameter" && typeof node.effectIndex === "number").every((node) => node.effectKind === undefined)).toBe(true)
  })

  it("records official source identity without putting DSL values in nodes", () => {
    const graph = createBlankSkillForgeGraph("official", "power_strike")
    expect(graph.source).toEqual({ kind: "official", skillId: "power_strike" })
    expect(graph.nodes.every((node) => !("compileOp" in node))).toBe(true)
  })

  it("represents each existing source effect without copying its DSL", () => {
    const graph = createBlankSkillForgeGraph("official", "mixed_skill", ["damage", "status", "heal"])
    const effects = graph.nodes.filter((node) => node.kind === "effect")
    expect(effects).toHaveLength(3)
    expect(effects.map((node) => node.effectIndex)).toEqual([0, 1, 2])
    expect(effects.map((node) => node.effectKind)).toEqual(["damage", "status", "heal"])
    expect(effects.every((node) => Object.keys(node).every((key) => !["amountFormula", "statusPayload", "targeting"].includes(key)))).toBe(true)
  })

  it("expands an official skill into readable DSL parameter slots", () => {
    const graph = createOfficialSkillForgeGraph("arrow_rain", {
      hpCost: 0,
      mpCost: 8,
      cooldown: 3,
      weaponRequirement: ["bow"],
      effects: [{
        kind: "damage",
        damageType: "physical",
        targeting: { targetScope: "enemy", targetPositions: ["vanguard", "midguard", "rearguard"], selectRule: "random", targetCount: { base: 4, formula: [], max: 7 } },
        amountFormula: { baseAdd: 0, terms: [{ attrKey: "physics_damage", coefficient: 0.85, perSkillLevel: 0.03 }] },
        damageSpec: { hitCount: { base: 1, formula: [] } }
      }]
    })
    const parameters = graph.nodes.filter((node) => node.kind === "parameter")
    expect(parameters.map((node) => node.parameterKind)).toEqual(expect.arrayContaining([
      "requirements", "mp_cost", "cooldown", "conditions", "target_scope", "target_count", "target_count_max", "target_direction", "select_rule", "target_positions", "formula", "hit_count"
    ]))
    expect(parameters.find((node) => node.parameterKind === "target_count")?.dslPath).toBe("effects.0.targeting.targetCount.base")
    expect(parameters.find((node) => node.parameterKind === "target_count_max")?.dslPath).toBe("effects.0.targeting.targetCount.max")
    expect(parameters.find((node) => node.parameterKind === "target_direction")?.dslPath).toBe("effects.0.targeting.secondarySeek")
    expect(graph.edges.some((edge) => edge.source === "source" && edge.target === "effect-1-conditions")).toBe(true)
    const source = graph.nodes.find((node) => node.id === "source")!
    const output = graph.nodes.find((node) => node.id === "output")!
    const firstEffect = graph.nodes.find((node) => node.id === "effect-1-conditions")!
    expect(source.position.y).toBeLessThan(firstEffect.position.y)
    expect(output.position.y).toBe(source.position.y)
    const branch = graph.nodes
      .filter((node) => node.effectIndex === 0 && (node.kind === "effect" || node.kind === "parameter"))
      .sort((left, right) => left.position.x - right.position.x)
    expect(branch.every((node, index) => index === 0 || node.position.x - branch[index - 1]!.position.x >= 300)).toBe(true)
  })

  it("keeps separate DSL effect rows far enough apart for attached material nodes", () => {
    const graph = createOfficialSkillForgeGraph("two_effects", {
      effects: [{ kind: "damage" }, { kind: "heal" }]
    })
    const first = graph.nodes.find((node) => node.id === "effect-1-conditions")!
    const second = graph.nodes.find((node) => node.id === "effect-2-conditions")!
    expect(second.position.y - first.position.y).toBeGreaterThanOrEqual(480)
  })

  it("preserves legendary and mythic batch counts across the corrected balance rollout", () => {
    expect(normalizeSkillForgeMaterialQuantity(50, { unitBasis: 15, maxBasisMultiplier: 4 }, "mythic")).toBe(15)
    expect(normalizeSkillForgeMaterialQuantity(150, { unitBasis: 15, maxBasisMultiplier: 4 }, "mythic")).toBe(45)
    expect(normalizeSkillForgeMaterialQuantity(15, { unitBasis: 200, maxBasisMultiplier: 4 }, "legendary")).toBe(200)
    expect(normalizeSkillForgeMaterialQuantity(60, { unitBasis: 200, maxBasisMultiplier: 4 }, "legendary")).toBe(800)
    expect(normalizeSkillForgeMaterialQuantity(75, { unitBasis: 1000, maxBasisMultiplier: 1 }, "legendary")).toBe(1000)
    expect(normalizeSkillForgeMaterialQuantity(30, { unitBasis: 15, maxBasisMultiplier: 4 }, "mythic")).toBe(30)
  })

  it("sends the caller-owned idempotency key when forging", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ok: true, data: { id: "revision:1" } }), {
      status: 200,
      headers: { "Content-Type": "application/json" }
    }))
    vi.stubGlobal("fetch", fetchMock)
    const graphHash = "a".repeat(64)
    await forgeSkillDraft("token", "draft:1", 7, graphHash, "skill-forge-retry-key")
    expect(fetchMock).toHaveBeenCalledOnce()
    expect((fetchMock.mock.calls[0]?.[1] as RequestInit).headers).toMatchObject({ "X-Idempotency-Key": "skill-forge-retry-key" })
    expect(JSON.parse(String((fetchMock.mock.calls[0]?.[1] as RequestInit).body))).toEqual({ expectedRevision: 7, expectedGraphHash: graphHash })
  })
})
