# 炼金继承来源项玩法设计

> 状态：v2 首版实现说明
> 范围：独立的炼金继承玩法，仅参考灌注概念，不复用灌注类型或 DSL。

## 核心体验

- 保留：玩家明确要求保留该来源项。
- 抑制：玩家明确要求移除该来源项。
- 不干预：玩家接受后端按介质概率保留或丢弃。
- 玩家能规划底线，但不能稳定毕业。
- UI 必须解释风险和收益，尤其是异源协同与同源畸变。

## 提交参数

```ts
type AlchemyInheritArgs = {
  fatherInstanceId: number
  fatherAffixIndex: number
  motherInstanceId: number
  motherAffixIndex: number
  carrierInstanceId: number
  mediumItemId: string
  geneChoices: Array<{
    parent: "father" | "mother"
    componentIndex: number
    sourceKey: string
    decision: "inherit" | "suppress" | "ignore"
  }>
}
```

旧 `includeGeneKeys` / `suppressGeneKeys` 不再接受。`geneChoices` 是内部 API 字段名；玩家界面统一称为“来源项”。

## 介质 DSL

`alchemy_inherit_mediums.json` 使用独立固定字段：

- `inheritSlots` / `suppressSlots`
- `ignoreKeepChance`
- `inheritCost` / `suppressCost`
- `hybridMergeDivisor`
- `inbreedMutationChance`
- `inbreedGoodChance`
- `inbreedGoodMultiplierRange`
- `inbreedBadMultiplierRange`
- `mutationChance`
- `precisionScaling`
- `luckyScaling`
- `stabilityScaling`

`minAlchemyLevel` 暂时保留，默认介质设为 1 便于测试。

## 后端规则

1. 左/右来源组件不预合并，按来源项级读取 `geneChoices`。
2. `inherit` 必保留并占继承额度；`suppress` 必移除并占抑制额度；`ignore` 按 `ignoreKeepChance` 独立 roll。
3. 保留后按属性处理：
   - 同属性、不同 `sourceKey`：异源融合，数值按 `1 + (N-1)*(hybridMergeDivisor-1)` 递减。
   - 同属性、同 `sourceKey`：先 roll 同源畸变。未畸变走普通融合；畸变分正向/负向。
4. 负数副作用按有利程度处理：
   - 正向畸变让负数更接近 0。
   - 负向畸变让负数更负。
5. 融合/畸变结果只保留一个主来源，用于后续遗传和 `affix_limits`。
6. 完整 `ancestry`、`sourceKeys`、`parents` 只用于展示、审计和复盘，不参与限流。
7. 输出继续写 `effectParams.alchemyComponents`，但每项为继承专属结构，并标记 `fusionKind`、`retentionReason`、主来源和完整来源。

## 普通词条与孤儿规则

- 亲本范围扩展到合法词条载体上的普通 affix；普通装备本体不作为首版亲本。
- 普通 affix 的类型按 `affix_definitions.type` 映射：`battle -> combat`，`work -> life`。
- 普通属性词条投影为 `affix:<affixId>:<attr>` 来源项；复杂效果投影为 `affix:<affixId>:effect` 整词条来源项。
- 普通复杂效果只保留/去重/并存，不参与数值融合或同源畸变倍率。
- 一旦普通 affix 参与继承，产出的子代 custom 词条标记 `inheritOrphan: true` / `sourceKind: inherit_orphan`。该词条继续生效，但不能再次作为继承亲本。
- 输出 `effectParams` 会合并保留来源的 `attrMods`、`battleHooks`、`effectMutations` 等 payload；战斗运行时按 payload 读取复杂效果。

## 三属性挂钩

- 炼金精度：降低融合除数，提升调和效率，但有上限。
- 炼金幸运：提高不干预保留率和正向畸变概率，但只提供边际改善。
- 炼金稳定：降低同源畸变概率，提高负向畸变倍率底线，降低坏结果幅度。

三属性不能让继承变成稳定毕业，只改善概率和风险边界。

## UI 原则

- 桌面端使用左来源/右来源左右对齐矩阵。
- 移动端按属性分组纵向展示。
- 每条来源项显示来源、当前值、值域和三态决策。
- 顶部实时显示继承额度、抑制额度、额外费用、潜在异源协同和同源风险。
- 风险提示必须在来源对旁边，而不是只放在规则说明里。
- 玩家界面禁用“基因”术语；内部实现和 API 可暂时保留 `geneChoices`。
- 普通 affix/orphan 风险必须在选择区和载体提示中明确说明。

## combat-model-vue3

- `alchemy_inherit_mediums` 独立编辑器，字段不进入 `alchemyMediumModifiers`。
- 审查视图展示额度、费用、不干预概率、融合递减、畸变概率/倍率、三属性缩放，以及普通 affix 参与后生成 orphan 的规则。
