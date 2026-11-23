# 战斗模型调参器（Vue3）

这是一个用于游戏战斗数值建模的 Vue3 + Vite 项目。它把原来 Excel 中复杂的公式拆成网页表单，让你可以可视化编辑：

- 每个属性的预算价值
- 技能：目标数量、伤害构成、作用位置、消耗、冷却、附加效果
- Buff：效果、目标数量、持续时间、覆盖率、叠层、是否可驱散
- 装备：基础属性、词条插槽、强化上限、每级强化属性、强化里程碑、稀有度、槽位
- 词条：效果、适用范围、稀有度、权重、冲突组
- 敌人：等级、阶级、定位权重、技能压力、奖励建议

最终可以导出以下 TSV 表格，直接复制到 Excel / WPS / Google Sheets：

- `Export_equipment`
- `Export_affix`
- `Export_enemy`
- `Export_combat`

## 启动

```bash
npm install
npm run dev
```

打开终端中显示的本地地址即可。

## 构建

```bash
npm run build
npm run preview
```

## 效果文本语法

大部分属性效果都使用同一种语法：

```text
physics_damage=12;crit_chance=3%;speed=18
```

等价于：

```json
{
  "physics_damage": 12,
  "crit_chance": 0.03,
  "speed": 18
}
```

支持分号、逗号、换行分隔。百分比可以写 `3%`，也可以写 `0.03`。

## 强化里程碑语法

```text
5:crit_chance=2%|10:physics_damage=6
```

会导出为：

```json
[
  { "level": 5, "attributes": { "crit_chance": 0.02 } },
  { "level": 10, "attributes": { "physics_damage": 6 } }
]
```

## 评分思想

项目使用统一预算 `score` 作为中间层，再通过 `goldPerBudget` 换算金币价值。

默认值只是起点，不是最终平衡。建议先调：

1. `属性价值`
2. `全局规则` 里的 `goldPerBudget`
3. `稀有度规则`
4. `槽位规则`
5. 敌人的定位权重

然后再批量调装备、技能和敌人。
