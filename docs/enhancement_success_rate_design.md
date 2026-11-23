# 强化成功率计算说明

## 1. 目标

本系统用于根据 **物品稀有度** 和 **物品最大强化等级**，自动计算每一级强化的基础成功率，并生成配置 JSON。

强化成功率不再为每件物品手动填写，而是由代码统一计算。

---

## 2. 输入参数

每件可强化物品只需要提供两个参数：

```ts
interface EnhancementInput {
  rarity: ItemRarity; // 物品稀有度
  maxLevel: number;   // 物品最大强化等级，例如 15 / 30 / 50
}
```

---

## 3. 稀有度枚举

推荐使用以下稀有度：

```ts
type ItemRarity =
  | "common"
  | "uncommon"
  | "rare"
  | "epic"
  | "legendary"
  | "mythic";
```

如果项目中已有自己的稀有度命名，可以在代码中做映射。

---

## 4. 设计原则

强化概率遵循以下原则：

1. 最大强化等级越低，满级成功率越高。
2. 最大强化等级越高，满级成功率越低。
3. 前期强化成功率较高，后期逐渐变难。
4. 稀有度越高，强化难度越高。
5. 稀有度不会直接降低成功率，而是放大失败率。
6. 因此前几级接近 100% 的体验不会被破坏。
7. 失败归 0、保护装备等逻辑不参与本公式，由强化结果处理逻辑单独实现。

---

## 5. 核心公式

### 5.1 强化进度

`level` 表示目标强化等级。

例如：

- `level = 1` 表示 `+0 -> +1`
- `level = 15` 表示 `+14 -> +15`

```ts
progress = (level - 1) / (maxLevel - 1)
```

当 `maxLevel <= 1` 时，直接认为成功率为 `1`。

---

### 5.2 满级最低成功率

最大强化等级越高，满级基础成功率越低。

```ts
minSuccessRate = clamp(
  0.60 - (maxLevel - 15) * (0.45 / 35),
  0.15,
  0.60
);
```

含义：

| 最大强化等级 | 满级基础成功率 |
|---:|---:|
| 15 | 60% |
| 30 | 约 40.7% |
| 50 | 15% |

---

### 5.3 曲线系数

最大强化等级越低，后期压力越集中。  
最大强化等级越高，概率下降更平滑。

```ts
curvePower = clamp(
  1.25 - (maxLevel - 15) * (0.50 / 35),
  0.75,
  1.25
);
```

---

### 5.4 基础成功率

```ts
baseRate =
  minSuccessRate +
  (1 - minSuccessRate) * (1 - Math.pow(progress, curvePower));
```

其中：

- `level = 1` 时，成功率接近 `100%`
- `level = maxLevel` 时，成功率等于 `minSuccessRate`

---

## 6. 稀有度修正

稀有度通过 **放大失败率** 来影响强化难度。

```ts
successRate = 1 - (1 - baseRate) * rarityFailMultiplier;
```

这样设计的好处是：

```txt
如果基础成功率是 100%，即使稀有度很高，最终仍然是 100%。
如果基础成功率较低，高稀有度会明显增加强化难度。
```

推荐稀有度失败率系数如下：

```ts
const RARITY_FAIL_MULTIPLIER: Record<ItemRarity, number> = {
  common: 1.00,
  uncommon: 1.06,
  rare: 1.12,
  epic: 1.20,
  legendary: 1.30,
  mythic: 1.45,
};
```

示例：

基础成功率为 `60%` 时：

| 稀有度 | 失败率系数 | 最终成功率 |
|---|---:|---:|
| common | 1.00 | 60% |
| uncommon | 1.06 | 57.6% |
| rare | 1.12 | 55.2% |
| epic | 1.20 | 52% |
| legendary | 1.30 | 48% |
| mythic | 1.45 | 42% |

---

## 7. 最终成功率限制

为了避免极端情况，最终成功率需要限制在合理范围内。

```ts
finalRate = clamp(successRate, 0.01, 1.00);
```

建议最低成功率为 `1%`，避免任何强化等级出现 `0%`。

---

## 8. 输出 JSON 格式

最终输出格式如下：

```ts
interface EnhancementSuccessRateConfig {
  overrides: {
    [level: string]: number; // 成功率，例如 0.03 / 0.1 / 1
  };
}
```

示例：

```json
{
  "overrides": {
    "1": 1,
    "2": 0.9961,
    "3": 0.9845,
    "4": 0.9654
  }
}
```

---

## 9. TypeScript 参考实现

```ts
type ItemRarity =
  | "common"
  | "uncommon"
  | "rare"
  | "epic"
  | "legendary"
  | "mythic";

interface EnhancementInput {
  rarity: ItemRarity;
  maxLevel: number;
}

interface EnhancementSuccessRateConfig {
  overrides: {
    [level: string]: number;
  };
}

const RARITY_FAIL_MULTIPLIER: Record<ItemRarity, number> = {
  common: 1.00,
  uncommon: 1.06,
  rare: 1.12,
  epic: 1.20,
  legendary: 1.30,
  mythic: 1.45,
};

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

function roundRate(value: number): number {
  return Math.round(value * 10000) / 10000;
}

/**
 * 计算指定目标强化等级的成功率。
 *
 * @param rarity 物品稀有度
 * @param maxLevel 物品最大强化等级
 * @param level 目标强化等级，例如 level=1 表示 +0 -> +1
 */
function calculateEnhancementSuccessRate(
  rarity: ItemRarity,
  maxLevel: number,
  level: number
): number {
  if (maxLevel <= 0) {
    throw new Error("maxLevel must be greater than 0");
  }

  if (level < 1 || level > maxLevel) {
    throw new Error(`level must be between 1 and ${maxLevel}`);
  }

  if (maxLevel === 1) {
    return 1;
  }

  const minSuccessRate = clamp(
    0.60 - (maxLevel - 15) * (0.45 / 35),
    0.15,
    0.60
  );

  const curvePower = clamp(
    1.75 - (maxLevel - 15) * (0.75 / 35),
    1.00,
    1.75
  );

  const progress = (level - 1) / (maxLevel - 1);

  const baseRate =
    minSuccessRate +
    (1 - minSuccessRate) * (1 - Math.pow(progress, curvePower));

  const rarityFailMultiplier = RARITY_FAIL_MULTIPLIER[rarity];

  const successRate =
    1 - (1 - baseRate) * rarityFailMultiplier;

  const finalRate = clamp(successRate, 0.01, 1.00);

  return roundRate(finalRate);
}

/**
 * 生成强化成功率 JSON。
 */
function generateEnhancementSuccessRateConfig(
  input: EnhancementInput
): EnhancementSuccessRateConfig {
  const { rarity, maxLevel } = input;

  const overrides: Record<string, number> = {};

  for (let level = 1; level <= maxLevel; level++) {
    overrides[String(level)] = calculateEnhancementSuccessRate(
      rarity,
      maxLevel,
      level
    );
  }

  return {
    overrides,
  };
}
```

---

## 10. 示例：普通物品，最大强化等级 15

输入：

```ts
generateEnhancementSuccessRateConfig({
  rarity: "common",
  maxLevel: 15,
});
```

大致输出：

```json
{
  "overrides": {
    "1": 1,
    "2": 0.9961,
    "3": 0.9867,
    "4": 0.973,
    "5": 0.9553,
    "6": 0.934,
    "7": 0.9092,
    "8": 0.8811,
    "9": 0.8498,
    "10": 0.8154,
    "11": 0.778,
    "12": 0.7377,
    "13": 0.6946,
    "14": 0.6487,
    "15": 0.6
  }
}
```

注意：上面的示例只展示格式。实际数值应以代码运行结果为准。

---

## 11. 示例：普通物品，最大强化等级 50

输入：

```ts
generateEnhancementSuccessRateConfig({
  rarity: "common",
  maxLevel: 50,
});
```

预期曲线大致表现：

| 目标强化等级 | 成功率 |
|---:|---:|
| +1 | 100% |
| +10 | 约 84% |
| +20 | 约 67% |
| +30 | 约 50% |
| +40 | 约 32% |
| +50 | 15% |

---

## 12. 失败归 0 与保护装备

本公式只负责计算强化成功率。

强化失败后的处理建议由业务逻辑控制：

```ts
if (success) {
  enhanceLevel += 1;
} else {
  if (hasProtectionItem) {
    // 推荐方案一：保护后等级不变
    enhanceLevel = enhanceLevel;

    // 推荐方案二：保护后只降低 1 级
    // enhanceLevel = Math.max(0, enhanceLevel - 1);
  } else {
    // 无保护装备时，失败归 0
    enhanceLevel = 0;
  }
}
```

推荐：

- 普通失败：强化等级归 0
- 使用保护装备失败：强化等级不变，或者只降低 1 级
- 保护装备消耗逻辑与成功率计算分离

---

## 13. 注意事项

### 13.1 不建议把强化经验等级放进这个公式

本版本只根据：

```txt
物品稀有度
物品最大强化等级
```

计算基础强化成功率。

如果以后需要加入强化经验等级，建议作为独立加成项处理，不要混入物品基础概率：

```ts
finalRate = baseItemRate + masteryBonus + itemBonus;
```

这样方便后续平衡。

---

### 13.2 不建议每件装备单独配置概率

除非是特殊活动装备、神器装备或者剧情装备，否则不建议每件装备单独写概率。

推荐统一使用本公式生成：

```json
{
  "overrides": {
    "1": 1,
    "2": 0.9961,
    "3": 0.9845
  }
}
```

这样装备数量增加后，强化系统仍然容易维护。

---

## 14. 推荐默认结论

默认情况下：

- `maxLevel = 15`：适合普通短线装备
- `maxLevel = 30`：适合中长期装备
- `maxLevel = 50`：适合毕业级长期追求装备
- 稀有度越高，强化越难
- +1 附近保持高成功率
- 满强化阶段形成主要消耗点
- 失败归 0 的惩罚通过保护装备缓冲
