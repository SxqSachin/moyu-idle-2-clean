# 传闻委托系统 (Rumor Commission System)

## 概述

传闻委托是猫猫村委托系统的特殊子类型，具备以下特征：

- 低概率随机出现（或保底触发）
- 通常一次性、不可刷新；第一阶段可放弃，后续阶段可显式禁止放弃
- 支持前置链（需先完成指定传闻）
- 支持保底机制（避免长时间未出现）
- 可配置完成后清除哪些传闻的完成记录（实现可重复循环链）

---

## 数据表

| 表 | 用途 |
|----|------|
| `player_cat_village_tasks` | 当前活跃委托（含传闻），字段 `not_refreshable` / `not_deletable` 控制操作权限 |
| `player_rumor_completion` | 玩家已完成的传闻 ID 记录（用于前置链判断和 oncePerAccount 去重） |
| `player_rumor_pity` | 保底计数器，每个玩家每条传闻一个计数 |

---

## DSL 字段（`cat_village_commissions.json`）

普通委托基础字段（weight / costs / rewardPool 等）之外，传闻专有字段：

```json
{
  "id": "cvc_rumor_example",
  "isRumor": true,
  "rumorBadge": "wolf_king",
  "oncePerAccount": true,
  "notRefreshable": true,
  "notDeletable": true,
  "pityThreshold": 48,
  "prereqRumorIds": ["cvc_rumor_example_prequel"],
  "clearCompletionIds": [],
  "costs": { "some_item": { "min": 5, "max": 5 } },
  "rewardPool": { "reward_item": { "min": 1, "max": 1 } },
  "reputationMin": 5, "reputationMax": 10,
  "weight": 1,
  "minRepLevel": 2
}
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `isRumor` | bool | 标记为传闻委托 |
| `rumorBadge` | string | 徽章标识（前端显示用，同系列传闻用同一值，如 `"wolf_king"`） |
| `oncePerAccount` | bool | `true` = 完成后永不再出现；`false` = 可重复 |
| `notRefreshable` | bool | `true` = 玩家不能手动刷新（换一个）此委托 |
| `notDeletable` | bool | `true` = 玩家不能放弃此委托；默认可放弃。传闻第一阶段应省略或设为 `false`，后续阶段应设为 `true` |
| `pityThreshold` | int | 保底次数，0=不保底。玩家自动获得 N 个委托后，此传闻必然出现 |
| `prereqRumorIds` | string[] | 前置传闻 ID 列表，全部完成后此传闻才进入候选池 |
| `clearCompletionIds` | string[] | 完成本传闻时，清除哪些传闻的完成记录并归零其保底计数（用于实现可重复循环链） |

---

## 刷新逻辑（`refreshCatVillageCommissions`）

每次自动刷新分三阶段，仅统计**自动批量生成**的 slot（手动"换一个"不计）：

### 阶段 0：准备

1. 查询当前活跃委托数，计算本批次生成量 `toAdd`
2. 加载玩家的传闻完成记录（`player_rumor_completion`）
3. 加载保底计数器（`player_rumor_pity`）
4. 过滤传闻候选池：`filterRumorCandidates`
   - 排除：`disabled`、已完成（`oncePerAccount`）、前置未满足、声望等级不足
5. 对所有有 `pityThreshold` 的候选传闻，计算新计数器：
   - `newCounter = min(current + toAdd, threshold)`（钉住上限，不溢出）
   - `newCounter >= threshold` → 加入强制触发队列

### 阶段 1：保底强制触发

- 按 ID 字典序处理强制触发队列
- 每触发一个：消耗 1 个 `toAdd` slot，写入委托，计数器归零
- 若 `toAdd = 0` 或 `activeRumorCount >= maxRumors`：保持计数在 threshold，下次有位置立即触发

### 阶段 2：随机触发（仅本批次无强制触发时）

- 掷骰 `rumorChance`（默认 15%），命中则随机从**未到保底**的候选传闻中选一个
- 命中后同样重置该传闻的保底计数

### 阶段 3：普通委托填充

- 用剩余 `toAdd` 生成普通委托

### 阶段 4：持久化保底计数

- 将 `updatedPity` map 中所有计数写入 `player_rumor_pity`

---

## 完成逻辑（`SubmitCatVillageCommission`）

1. 消耗费用物品，发放奖励物品
2. 标记任务为 `completed`
3. 若 `oncePerAccount = true`：写入 `player_rumor_completion`
4. 遍历 `clearCompletionIds`：
   - 删除对应条目的 `player_rumor_completion` 记录
   - 将对应条目的 `player_rumor_pity.counter` 归零

---

## 前端展示

- 传闻委托**始终置顶**显示（`sortedTasks` computed 按 `isRumor` 降序，次级按 `id` 降序）
- 传闻卡片：金色边框、"传闻"徽章
- 有 `pityThreshold` 的传闻显示**保底进度条**（`pityCounter / pityThreshold`）
- `notRefreshable = true` → 隐藏"换一个"按钮
- `notDeletable = true` → 隐藏"放弃"按钮
- 传闻第一阶段默认可放弃；提交第一阶段后出现的后续阶段应通过 `notDeletable = true` 禁止放弃

---

## 在 combat-model-vue3 中创建新传闻链

### 步骤 1：规划传闻链

设计每个阶段的叙事、材料消耗、奖励物品、前后置关系。

示例三阶段链：
```
A → B → C
A：收集证据（消耗 ordinary_material ×10）→ 奖励线索道具
B（前置A）：前往侦查（消耗线索道具 ×1）→ 奖励解锁道具
C（前置B）：最终决战准备（消耗解锁道具 ×1）→ 奖励触发物品
```

#### 关于任务物品的设计原则

**不要用"照片"作为万能触发物。** 每条传闻链的任务物品应贴合故事逻辑，让玩家感受到叙事而非机械流程。

| 坏例子 | 好例子 |
|--------|--------|
| `wolf_king_photo`（狼王照片）| ✓ 可以，因为故事里"老猎人多年前拍的模糊照片"有戏剧张力 |
| `super_berry_bird_photo`（超级浆果鸟照片）| ✗ 无聊，换成 `super_sweet_berry`（超甜大浆果）——浆果鸟喜欢吃，拿着就能引出它 |
| `boss_X_photo` | ✗ 直接复用"照片"格式，缺乏故事味 |

问自己：**玩家拿着这个道具进副本，这个行为在世界观里说得通吗？** 
- 老猎人的照片 → 带着线索认脸，说得通  
- 超甜大浆果 → 用食物引诱贪吃的大鸟，说得通  
- 随手拍的照片 → 拿照片打 boss，说不通  

触发物品可以是：诱饵食物、特殊气味的素材、古老召唤符文、某生物独有的领地标记……只要世界观内有逻辑即可。

---

## 传闻设计规范（强制）

每条传闻链必须满足以下要求：

### 1. 触发物品（trigger item）必须展示战斗参数

触发物品（`carryableInCombat: true` + `battleUsableItemId` 指向 `isTrigger: true` 条目）在物品弹窗中会自动展示以下信息，所有字段必须填写：

| `battle_usable_items.json` 字段 | 说明 |
|---|---|
| `dungeonIds` | 在哪些副本内生效，如 `["base_1"]` |
| `spawnEnemyIds` | 可召唤的敌人 ID 列表 |
| `minWave` | 最早触发波次（建议 ≥ 1，避免第一波即出现） |
| `perWaveChance` | 每波触发概率（如 `0.05` = 5%） |

前端 `BattleUsableItemStaticInfo.vue` 检测到 `isTrigger: true` 时，自动渲染"触发条件"区块，副本名与敌人名通过 i18n 自动翻译。

### 2. Boss 设计标准

每条传闻链的 Boss 必须具备：

- **两个专有技能**：在 `skills.json` 中定义，ID 以 `{boss_id_without_001}_` 前缀区分，如 `super_berry_bird_dive`。技能设计须体现该生物的行为特征，避免套用通用模板。

- **一个专有掉落物**：在 `items.json` 中定义，类型建议为 `material`（可作为传说装备合成材料）或 `collectible`（触发永久属性加成）。掉落名称和描述须与 Boss 生物学逻辑自洽。

### 3. 关联关系清单

一条完整的传闻链创建后，需确认以下所有资源均已创建：

```
传闻委托        → cat_village_commissions.json（isRumor: true）
  ↓ 奖励
触发物品        → items.json（trigger_item 类型，carryableInCombat: true）
  ↓ battleUsableItemId
战斗可用条目    → battle_usable_items.json（isTrigger: true，含副本/敌人/概率）
  ↓ spawnEnemyIds
Boss 敌人       → enemies.json（type: "boss"，含专有技能 ID）
  ↓ skillIds
专有技能 ×2     → skills.json
  ↓ drops
专有掉落物 ×1   → items.json（material 或 collectible）
```

每个环节均须补全 i18n（zh-CN.ts / en-US.ts）。

### 步骤 2：在编辑器中打开 `cat_village_commissions` 表

进入 combat-model-vue3，左侧菜单选择 **猫猫村委托**。

### 步骤 3：创建第一阶段传闻（A）

点击 **新建** 或复制已有传闻，填写以下字段：

| 字段 | 填写 |
|------|------|
| id | `cvc_rumor_<系列名>_prequel`，如 `cvc_rumor_wolf_king_prequel` |
| isRumor | `true` |
| rumorBadge | 系列标识，如 `wolf_king` |
| oncePerAccount | 视设计而定（一次性填 `true`，可重复填 `false`） |
| notRefreshable | `true`（推荐） |
| notDeletable | 省略或 `false`（第一阶段允许放弃） |
| pityThreshold | 保底次数，如 `48`（4小时×6次/批 = 约2天） |
| prereqRumorIds | `[]`（第一阶段无前置） |
| clearCompletionIds | `[]`（第一阶段通常不清除任何记录） |
| costs | 消耗物品，格式：`{ "item_id": { "min": N, "max": N } }` |
| rewardPool | 奖励物品，格式：`{ "item_id": { "min": N, "max": N } }` |
| weight | 传闻权重，通常填 `1` |
| minRepLevel | 最低声望等级要求 |

### 步骤 4：创建后续阶段（B、C…）

- `prereqRumorIds` 填入上一阶段的 ID
- `notDeletable` 设为 `true`，防止链条中途放弃导致互斥组或完成记录状态异常
- `costs` 改为消耗上阶段奖励的道具
- 最终阶段若希望链条可重复：
  - 设 `oncePerAccount: false`
  - 设 `clearCompletionIds: ["cvc_rumor_<系列>_prequel", "cvc_rumor_<系列>_stage2", ...]`（清除所有前阶段，含自身若需要）
  - 这样完成最终阶段后，整条链从第一阶段重新开始

### 步骤 5：在 enemies.json 中创建目标敌人（若需要）

若传闻奖励是触发物品，需在 enemies.json 创建对应 boss 敌人，并在 battle_usable_items.json 中创建触发物品条目（参见下方）。

### 步骤 6：创建触发物品（可选）

在 **战斗可用物品** 表中新建条目，勾选 **被动触发物品**，填写：

| 字段 | 说明 |
|------|------|
| isTrigger | `true` |
| dungeonIds | 适用副本，如 `["base_1"]` |
| spawnEnemyIds | 可召唤的敌人 ID 列表 |
| minWave | 最早触发波次（建议 ≥1） |
| perWaveChance | 每波触发概率，如 `0.30` |

对应的 item 需设 `carryableInCombat: true`，`battleUsableItemId` 指向此条目。

### 步骤 7：创建收藏品（可选）

在 **收藏品** 表中新建条目，填写 `itemId`（战利品道具 ID）、`playerMods` / `kittyMods`（永久属性加成，键为属性 key，值为加成量）。

### 步骤 8：导出并重建游戏数据

在项目根目录执行：
```bash
make game-data
```

这将重新生成 `generated/` 目录下所有 chunk 文件。

### 步骤 9：补全 i18n

执行 `make game-data` 后，`web/src/locales/zh-CN.ts` 和 `en-US.ts` 会自动生成空白的 i18n key 条目（物品、敌人、技能名称和描述）。搜索空字符串 `"name": ""` 补全对应的文本。

---

## 现有传闻链示例

### 狼王系列（两阶段，带前置）

```
cvc_rumor_wolf_king_prequel
  isRumor: true, oncePerAccount: true
  pityThreshold: 48
  costs: strange_wolf_fur ×8  （base_1 波次掉落，1% 概率）
  rewardPool: wolf_king_trail_clue ×1

    ↓ prereqRumorIds

cvc_rumor_wolf_king
  isRumor: true, oncePerAccount: true
  costs: wolf_king_trail_clue ×1
  rewardPool: wolf_king_photo ×1  （触发物品）

    ↓ 携入 base_1（30%/波召唤 wolf_king_001）

wolf_king_001 Boss
  技能：狂野撕咬（单体重击+防御削弱）/ 王者嚎叫（自身增益+全体恐慌）/ 撕裂爪击（前中排3目标）
  掉落：wolf_king_head_trophy 40% / wolf_king_claw_trophy 40% / wolf_king_fang 20%

    ↓ 仓库使用战利品

收藏品：wolf_king_head（玩家+1力量）/ wolf_king_claw（猫咪+1力量）
传说材料：wolf_king_fang（用于合成传说武器）
```

### 超级浆果鸟系列（单阶段，循环可重复）

```
cvc_rumor_super_berry_bird
  isRumor: true, oncePerAccount: true
  pityThreshold: 48
  costs: gold_coin ×20000  （直接购买情报，无材料收集前置）
  rewardPool: super_sweet_berry ×1  （触发物品，贴合故事：大鸟爱吃的诱饵）
  clearCompletionIds: ["cvc_rumor_super_berry_bird"]  （含自身 → 完成后可再触发）

    ↓ 携入 base_1（30%/波召唤 super_berry_bird_001）

super_berry_bird_001 Boss  （待实现）

触发物品设计说明：
  选择"超甜大浆果"而非"照片"，因为逻辑链条清晰：
  大鸟贪吃 → 特殊浆果引诱 → 带着诱饵进副本 → 大鸟循香而来
```
