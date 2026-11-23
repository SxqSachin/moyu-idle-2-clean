# 经济系统模型（当前阶段）

## 1. 锚点

- 时间价值锚点：`1 秒 = 1～1.5 金币`
- 成长经验锚点：`8 秒 = 1 经验`

> 以上锚点用于策划衡量动作/物品价值，不要求本阶段后端统一自动结算。

## 2. 价值与经验口径

### 2.1 动作价值（概念公式）

- 基础价值：

`baseValue = baseTime × rarityMultiplier × industryMultiplier × difficultyMultiplier`

- 基础经验：

`baseExp = (baseTime × 0.1) × rarityMultiplier × industryMultiplier × difficultyMultiplier`

### 2.2 本阶段执行口径

- 动作侧暂不接入稀有度倍率，固定视为：`rarityMultiplier = 1`
- 物品基础价值由策划直接填写，不由运行时强制推导
- 运行时需要使用时（如回收价、NPC 收购锚点）可直接读取导表字段

## 3. 示例

### 3.1 一级产物

- 伐木：`8 秒 -> 1 木头`
- 若难度倍率 `1.0`：
  - 价值约 `8 × 1.0 × (1～1.5) = 8～12`
  - 经验约 `(8 / 8) × 1.0 = 1`

### 3.2 高门槛动作

- 挖掘黑曜石：`64 秒 -> 1 黑曜石`，难度倍率 `2.5`
- 价值约 `64 × 2.5 × (1～1.5) = 160～240`
- 经验约 `(64 / 8) × 2.5 = 20`

### 3.3 二级产物（示意）

- 木剑配方消耗木头与石头，同时有制作耗时
- 策划可综合材料成本 + 制作时间 + 难度后，把结果写入 `items.economyBaseValue`
- 运行时不强制重算，直接消费该字段

## 4. 数据字段（已落表）

- `items.economyBaseValue`
  - 物品基础价值锚点，供回收/市场/NPC 定价参考
- `actions.economyIndustryTier`
  - 动作工业层级倍率
- `actions.economyDifficultyMultiplier`
  - 动作难度倍率
- `rarity_definitions.economyMultiplier`
  - 稀有度倍率（当前动作侧未强制使用）

## 5. 当前实现边界

- 已实现：
  - Excel 列识别
  - 构建校验与默认值
  - 输出到 `game-data.json` / Go / TS 生成类型
  - 回收运行时系数口径：
    - `global-config.economy.itemRecycleGoldCoefficient`：模板一致时保留比例
    - `global-config.economy.itemRecycleAffixMismatchExtraCoefficient`：仅词条不一致时附加保留比例（最终系数=`itemRecycleGoldCoefficient + itemRecycleAffixMismatchExtraCoefficient - 1`）
- 未实现（后续按业务接入）：
  - 全局统一运行时价格/经验自动结算
  - 市场/NPC/回收策略的统一动态模型
