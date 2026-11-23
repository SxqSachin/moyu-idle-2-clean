import { describe, expect, it } from "vitest"
import type { SpecializationNode } from "../src/services/specialization"
import {
  buildSpecializationDepthMap,
  collectSpecializationAncestorIds,
  minimalSpecializationPrerequisiteIds,
  specializationNodePrerequisiteIds
} from "../src/services/specialization-route"

function node(
  id: string,
  requirements: SpecializationNode["requirements"] = { all: [], anyOf: [] }
): SpecializationNode {
  return {
    id,
    kind: id.includes("join") ? "confluence" : id.includes("root") ? "root" : "technique",
    schoolIds: id.includes("other") ? ["perception"] : ["strength"],
    nameKey: `${id}.name`,
    descKey: `${id}.desc`,
    requirements,
    levels: [{ level: 1, cost: 1, grants: [] }],
    ui: {}
  }
}

describe("specialization route projection", () => {
  it("collects the complete recursive prerequisite closure, including every any-of route", () => {
    const nodes = [
      node("strength_root"),
      node("strength_left", { all: [{ kind: "node_rank", nodeId: "strength_root" }], anyOf: [] }),
      node("strength_right", { all: [{ kind: "node_rank", nodeId: "strength_root" }], anyOf: [] }),
      node("other_root"),
      node("other_tech", { all: [{ kind: "node_rank", nodeId: "other_root" }], anyOf: [] }),
      node("join", {
        all: [
          { kind: "node_rank", nodeId: "strength_root" },
          { kind: "node_rank", nodeId: "other_root" }
        ],
        anyOf: [[
          { kind: "node_rank", nodeId: "strength_left" },
          { kind: "node_rank", nodeId: "strength_right" }
        ], [
          { kind: "node_rank", nodeId: "other_tech" }
        ]]
      })
    ]

    expect(Array.from(collectSpecializationAncestorIds(nodes, "join")).sort()).toEqual([
      "other_root",
      "other_tech",
      "strength_left",
      "strength_right",
      "strength_root"
    ])
    expect(specializationNodePrerequisiteIds(nodes[5]!)).toHaveLength(5)
  })

  it("lays filtered routes into deterministic dependency depths and fails cycles closed", () => {
    const nodes = [
      node("strength_root"),
      node("strength_mid", { all: [{ kind: "node_rank", nodeId: "strength_root" }], anyOf: [] }),
      node("strength_end", { all: [{ kind: "node_rank", nodeId: "strength_mid" }], anyOf: [] }),
      node("cycle_a", { all: [{ kind: "node_rank", nodeId: "cycle_b" }], anyOf: [] }),
      node("cycle_b", { all: [{ kind: "node_rank", nodeId: "cycle_a" }], anyOf: [] })
    ]
    const depths = buildSpecializationDepthMap(nodes)

    expect(depths.get("strength_root")).toBe(0)
    expect(depths.get("strength_mid")).toBe(1)
    expect(depths.get("strength_end")).toBe(2)
    expect(Number.isFinite(depths.get("cycle_a"))).toBe(true)
    expect(Number.isFinite(depths.get("cycle_b"))).toBe(true)
  })

  it("removes a redundant root-to-terminal edge while preserving the real route", () => {
    const root = node("strength_root")
    const technique = node("strength_mid", {
      all: [{ kind: "node_rank", nodeId: "strength_root" }],
      anyOf: []
    })
    const terminal = node("strength_end", {
      all: [
        { kind: "node_rank", nodeId: "strength_root" },
        { kind: "node_rank", nodeId: "strength_mid" }
      ],
      anyOf: []
    })

    expect(specializationNodePrerequisiteIds(terminal)).toEqual(["strength_root", "strength_mid"])
    expect(minimalSpecializationPrerequisiteIds([root, technique, terminal], terminal)).toEqual(["strength_mid"])
    expect(minimalSpecializationPrerequisiteIds([root, technique, terminal], technique)).toEqual(["strength_root"])
  })

  it("keeps alternative branch edges but removes roots already implied by them", () => {
    const root = node("strength_root")
    const left = node("strength_left", {
      all: [{ kind: "node_rank", nodeId: "strength_root" }],
      anyOf: []
    })
    const right = node("strength_right", {
      all: [{ kind: "node_rank", nodeId: "strength_root" }],
      anyOf: []
    })
    const terminal = node("strength_end", {
      all: [{ kind: "node_rank", nodeId: "strength_root" }],
      anyOf: [[
        { kind: "node_rank", nodeId: "strength_left" },
        { kind: "node_rank", nodeId: "strength_right" }
      ]]
    })

    expect(minimalSpecializationPrerequisiteIds([root, left, right, terminal], terminal)).toEqual([
      "strength_left",
      "strength_right"
    ])
  })

  it("keeps a mandatory edge when only some alternative routes imply it", () => {
    const root = node("strength_root")
    const descendant = node("strength_left", {
      all: [{ kind: "node_rank", nodeId: "strength_root" }],
      anyOf: []
    })
    const unrelated = node("other_tech")
    const terminal = node("strength_end", {
      all: [{ kind: "node_rank", nodeId: "strength_root" }],
      anyOf: [[
        { kind: "node_rank", nodeId: "strength_left" },
        { kind: "node_rank", nodeId: "other_tech" }
      ]]
    })

    expect(minimalSpecializationPrerequisiteIds([root, descendant, unrelated, terminal], terminal)).toEqual([
      "strength_root",
      "strength_left",
      "other_tech"
    ])
  })
})
