# 展台 (Showcase) Feature — 设计文档

## 概述

玩家希望在个人资料页面展示自己仓库中的稀有/得意物品，让其他玩家看到。现有玩家信息查询接口 (`GET /api/player/profile/:userId`) 返回 Kitties、技能、收藏品数、成就、声望等信息。本功能在此基础上扩展"展台"模块。

## 需求

1. 新增数据库表 `player_showcase` 存储展示物品（最多 3 个）
2. 在 profile API 中返回展示物品信息（无新读取接口）
3. 新增 `POST /api/showcase/set` 批量设置展示物品
4. 前端 `UserProfileDialogHost.vue` Overview tab 新增 ShowcaseCabinet 区块
5. 前端 `ItemDetailActionBar.vue` 新增"展示/取消展示"按钮
6. 展示操作集成到现有仓库物品操作按钮中

## 设计决策

| 决策点 | 选择 | 原因 |
|--------|------|------|
| 存储方案 | 独立表 `player_showcase` | 展示是纯展示功能，物品保持 `state='inventory'`，不污染物品状态机 |
| 设置 API | 批量替换（发送完整列表） | 客户端发 `instanceIds[]`，服务端 delete-all + insert-all，避免 add/remove 的顺序竞争 |
| 读取过滤 | 懒验证（JOIN 时过滤） | `GetShowcase` JOIN `player_item_instances` 过滤 `state='inventory' AND user_id=owner`，装备中/拍卖中的物品自动不显示 |
| 删除清理 | `ON DELETE CASCADE` | 物品实例被删除（回收、市场消耗完）时自动清理展示记录 |
| 展示上限 | 最多 3 个 | 服务端验证 `len(instanceIDs) <= 3` |

## 数据库设计

### 迁移文件

新建 `server/schema/migrations/20260623_player_showcase.sql`

```sql
-- 展台：玩家展示仓库物品的独立表
CREATE TABLE player_showcase (
  user_id       BIGINT      NOT NULL,
  instance_id   BIGINT      NOT NULL,
  display_order SMALLINT    NOT NULL DEFAULT 0,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, display_order),
  UNIQUE  (user_id, instance_id),
  FOREIGN KEY (instance_id) REFERENCES player_item_instances(instance_id)
    ON DELETE CASCADE
);
CREATE INDEX idx_player_showcase_user ON player_showcase(user_id);
```

### 表结构说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `user_id` | BIGINT | 玩家 ID |
| `instance_id` | BIGINT | 物品实例 ID（引用 `player_item_instances`） |
| `display_order` | SMALLINT | 展示顺序（0, 1, 2） |
| `created_at` | TIMESTAMPTZ | 创建时间 |

**约束**：
- 主键 `(user_id, display_order)` — 同一玩家每个展示位最多一个物品
- 唯一 `(user_id, instance_id)` — 同一物品不能重复展示
- 外键 `ON DELETE CASCADE` — 物品实例删除时自动清理

## 后端设计

### 1. Store 层

新建文件：`server/internal/store/store_showcase.go`

```go
package store

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5"
)

// ShowcaseItem 展示物品数据（供 Profile API 返回）
type ShowcaseItem struct {
    InstanceID     int64  `json:"instanceId"`
    TemplateItemID string `json:"templateItemId"`
    EnhanceLevel   int    `json:"enhanceLevel"`
    Quantity       int64  `json:"quantity"`
}

// GetShowcase 查询玩家展示物品
// 懒过滤：只返回 state='inventory' 且属于该用户的物品
func (s *Store) GetShowcase(ctx context.Context, userID int64) ([]ShowcaseItem, error) {
    rows, err := s.pool.Query(ctx, `
        SELECT sc.instance_id, i.template_item_id, i.enhance_level, i.quantity
        FROM player_showcase sc
        JOIN player_item_instances i ON i.instance_id = sc.instance_id
        WHERE sc.user_id = $1
          AND i.user_id = $1
          AND i.state = 'inventory'
        ORDER BY sc.display_order
    `, userID)
    if err != nil {
        return nil, fmt.Errorf("showcase get: %w", err)
    }
    defer rows.Close()

    var items []ShowcaseItem
    for rows.Next() {
        var item ShowcaseItem
        if err := rows.Scan(&item.InstanceID, &item.TemplateItemID, &item.EnhanceLevel, &item.Quantity); err != nil {
            return nil, fmt.Errorf("showcase scan: %w", err)
        }
        items = append(items, item)
    }
    return items, rows.Err()
}

// SetShowcase 批量设置展示物品（事务）
// instanceIDs 为完整列表（0-3 个），服务端执行 delete-all + insert-all
func (s *Store) SetShowcase(ctx context.Context, userID int64, instanceIDs []int64) error {
    if len(instanceIDs) > 3 {
        return ErrShowcaseTooMany
    }

    // 检查重复
    seen := make(map[int64]struct{}, len(instanceIDs))
    for _, id := range instanceIDs {
        if _, ok := seen[id]; ok {
            return ErrShowcaseDuplicate
        }
        seen[id] = struct{}{}
    }

    return s.withTx(ctx, func(tx pgx.Tx) error {
        if err := s.lockUserTx(ctx, tx, userID); err != nil {
            return err
        }

        // 验证所有 instance 属于该用户且 state='inventory'
        if len(instanceIDs) > 0 {
            var validCount int64
            if err := tx.QueryRow(ctx, `
                SELECT COUNT(*)
                FROM player_item_instances
                WHERE user_id = $1
                  AND state = 'inventory'
                  AND instance_id = ANY($2)
            `, userID, instanceIDs).Scan(&validCount); err != nil {
                return fmt.Errorf("showcase validate: %w", err)
            }
            if int(validCount) != len(instanceIDs) {
                // 区分"不属于用户"和"不在 inventory 状态"
                var ownedCount int64
                if err := tx.QueryRow(ctx, `
                    SELECT COUNT(*)
                    FROM player_item_instances
                    WHERE user_id = $1
                      AND instance_id = ANY($2)
                `, userID, instanceIDs).Scan(&ownedCount); err != nil {
                    return fmt.Errorf("showcase validate owned: %w", err)
                }
                if ownedCount != int64(len(instanceIDs)) {
                    return ErrShowcaseInstanceNotOwned
                }
                return ErrShowcaseInstanceNotInventory
            }
        }

        // 删除旧展示
        if _, err := tx.Exec(ctx, `DELETE FROM player_showcase WHERE user_id = $1`, userID); err != nil {
            return fmt.Errorf("showcase delete: %w", err)
        }

        // 插入新展示
        for i, instanceID := range instanceIDs {
            if _, err := tx.Exec(ctx, `
                INSERT INTO player_showcase (user_id, instance_id, display_order)
                VALUES ($1, $2, $3)
            `, userID, instanceID, i); err != nil {
                return fmt.Errorf("showcase insert: %w", err)
            }
        }

        return nil
    })
}
```

### 2. 错误哨兵

添加到 `server/internal/store/errors.go`：

```go
var (
    // ... 现有错误

    ErrShowcaseTooMany             = errors.New("showcase: too many items (max 3)")
    ErrShowcaseInstanceNotOwned    = errors.New("showcase: instance not owned by user")
    ErrShowcaseInstanceNotInventory = errors.New("showcase: instance not in inventory state")
    ErrShowcaseDuplicate           = errors.New("showcase: duplicate instance id")
)
```

### 3. HTTP Handler

新建文件：`server/internal/httpapi/server_showcase.go`

```go
package httpapi

import (
    "github.com/gofiber/fiber/v2"
    "mysecondwebgame/server/internal/store"
)

type showcaseSetRequest struct {
    InstanceIDs []int64 `json:"instanceIds"`
}

func (s *Server) handleShowcaseSet(c *fiber.Ctx) error {
    userID, err := s.authUserID(c)
    if err != nil {
        return s.writeAPIError(c, fiber.StatusUnauthorized, codeAuthInvalidToken, "invalid bearer token", err.Error())
    }

    var req showcaseSetRequest
    if err = c.BodyParser(&req); err != nil {
        return s.writeAPIError(c, fiber.StatusBadRequest, "SHOWCASE_INVALID_PAYLOAD", "invalid payload", err.Error())
    }

    if err = s.deps.Store.SetShowcase(c.Context(), userID, req.InstanceIDs); err != nil {
        if errors.Is(err, store.ErrShowcaseTooMany) {
            return s.writeAPIError(c, fiber.StatusBadRequest, "SHOWCASE_TOO_MANY", "showcase full, max 3 items", "")
        }
        if errors.Is(err, store.ErrShowcaseDuplicate) {
            return s.writeAPIError(c, fiber.StatusBadRequest, "SHOWCASE_DUPLICATE", "duplicate instance id", "")
        }
        if errors.Is(err, store.ErrShowcaseInstanceNotOwned) {
            return s.writeAPIError(c, fiber.StatusForbidden, "SHOWCASE_NOT_OWNED", "instance not owned", "")
        }
        if errors.Is(err, store.ErrShowcaseInstanceNotInventory) {
            return s.writeAPIError(c, fiber.StatusConflict, "SHOWCASE_NOT_INVENTORY", "instance not in inventory", "")
        }
        return s.writeAPIError(c, fiber.StatusInternalServerError, "SHOWCASE_SET_FAILED", "set showcase failed", err.Error())
    }

    // 清除 profile 缓存
    s.playerProfileMu.Lock()
    delete(s.playerProfileMem, userID)
    s.playerProfileMu.Unlock()

    // 触发 WS 同步
    s.syncAfterMutation(mutationSyncScope{
        UserIDs:     []int64{userID},
        PlayerState: true,
        Inventory:   true,
    })

    return s.writeAPISuccess(c, fiber.StatusOK, fiber.Map{"ok": true})
}
```

### 4. 路由注册

修改 `server/internal/httpapi/server_http_base.go`，在 `protected` 组中添加：

```go
protected.Post("/showcase/set", s.handleShowcaseSet)
```

建议放在 `wardrobe` 相关路由附近（约 L142-144）。

### 5. Profile API 扩展

修改 `server/internal/httpapi/server_player_profile.go` 的 `handlePlayerProfileGet`：

```go
// 在 reputation 查询之后添加：
showcase, err := s.deps.Store.GetShowcase(c.Context(), userID)
if err != nil {
    return s.writeAPIError(c, fiber.StatusInternalServerError, "PLAYER_PROFILE_SHOWCASE_FAILED", "get showcase failed", err.Error())
}

payload := fiber.Map{
    // ... 现有字段
    "showcase": showcase,  // 新增
}
```

## 前端设计

### 1. TypeScript 类型

修改 `web/src/stores/local-user.ts`：

```typescript
// 新增类型
export type PlayerShowcaseItem = {
  instanceId: number
  templateItemId: string
  enhanceLevel: number
  quantity: number
}

// PlayerProfile 增加字段
export type PlayerProfile = {
  // ... 现有字段
  showcase?: PlayerShowcaseItem[]
}
```

新增 action：

```typescript
async setShowcase(token: string, instanceIds: number[]): Promise<void> {
  if (!token) throw new Error("invalid showcase request")
  await apiPost("/api/showcase/set", token, { instanceIds })
  
  // 清除当前用户的 profile 缓存
  const currentUserId = this.currentUserId  // 需要新增或从其他地方获取
  if (currentUserId > 0) {
    delete this.profileByUserID[currentUserId]
  }
}
```

### 2. ShowcaseCabinet 组件

新建文件：`web/src/components/game/item/ShowcaseCabinet.vue`

```vue
<template>
  <div class="flex flex-wrap justify-center gap-3">
    <div
      v-for="item in items"
      :key="item.instanceId"
      class="showcase-item"
    >
      <ItemCard
        :item-id="item.templateItemId"
        :instance-id="item.instanceId"
        :quantity="item.quantity"
        :hide-label="false"
        :width="80"
        :height="100"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import type { PlayerShowcaseItem } from "@stores/local-user"
import ItemCard from "./Card.vue"

defineProps<{
  items: PlayerShowcaseItem[]
}>()
</script>

<style scoped>
.showcase-item {
  transition: transform 0.2s ease;
}
.showcase-item:hover {
  transform: translateY(-2px);
}
</style>
```

### 3. UserProfileDialogHost 集成

修改 `web/src/components/common/UserProfileDialogHost.vue`：

**模板**（在 Overview tab 的 Kitties section 之前插入）：

```vue
<!-- Showcase section (only when items exist) -->
<div v-if="showcaseItems.length > 0" class="rounded-lg border border-[var(--app-border)] p-3">
  <div class="mb-2 text-sm font-semibold">{{ t("game.userProfileShowcaseTitle") }}</div>
  <ShowcaseCabinet :items="showcaseItems" />
</div>
```

**Script**：

```typescript
import ShowcaseCabinet from "@components/game/item/ShowcaseCabinet.vue"

const showcaseItems = computed(() => profile.value?.showcase || [])
```

### 4. ItemDetailActionBar 扩展

修改 `web/src/components/game/item/ItemDetailActionBar.vue`：

**Props 新增**：

```typescript
withDefaults(
  defineProps<{
    // ... 现有 props
    canShowcase?: boolean
    canUnshowcase?: boolean
  }>(),
  {
    // ... 现有默认值
    canShowcase: false,
    canUnshowcase: false
  }
)
```

**Emits 新增**：

```typescript
defineEmits<{
  // ... 现有 emits
  (e: "showcase"): void
  (e: "unshowcase"): void
}>()
```

**模板**（在 market/recycle 行中添加）：

```vue
<div v-if="canShowMarket || canShowRecycle || canShowcase || canUnshowcase" class="flex flex-row items-center gap-2 justify-between">
  <!-- ... 现有按钮 ... -->
  <el-button class="w-full ml-0!" v-if="canShowcase" @click="$emit('showcase')">
    {{ t("game.showcaseAdd") }}
  </el-button>
  <el-button class="w-full ml-0!" v-if="canUnshowcase" type="warning" @click="$emit('unshowcase')">
    {{ t("game.showcaseRemove") }}
  </el-button>
</div>
```

### 5. ItemDetailPanel 集成

修改 `web/src/components/game/item/ItemDetailPanel.vue`：

**新增 computed**：

```typescript
import { useLocalUserStore } from "@stores/local-user"

const localUserStore = useLocalUserStore()

// 获取当前登录用户的 profile（用于判断展示状态）
const myProfile = computed(() => {
  const myUserId = gameStore.userId  // 需要从 gameStore 获取当前用户 ID
  return myUserId > 0 ? localUserStore.getProfile(myUserId) : null
})

const isShowcased = computed(() => {
  const instanceId = currentInstanceId()
  if (!instanceId || !myProfile.value) return false
  const showcase = myProfile.value.showcase || []
  return showcase.some(s => s.instanceId === instanceId)
})

const canShowcase = computed(() => {
  if (!currentInstance.value || currentInstance.value.state !== "inventory") return false
  if (!canOperateCurrentInstance.value) return false
  if (isShowcased.value) return false
  const showcase = myProfile.value?.showcase || []
  return showcase.length < 3
})

const canUnshowcase = computed(() => {
  return isShowcased.value && canOperateCurrentInstance.value
})

const canShowShowcaseButton = computed(() => canShowcase.value || canUnshowcase.value)
```

**新增 handler**：

```typescript
async function showcaseItem() {
  const instanceId = currentInstanceId()
  if (!store.token || !instanceId || !canShowcase.value) return
  try {
    const current = myProfile.value?.showcase || []
    const newIds = [...current.map(s => s.instanceId), instanceId]
    await localUserStore.setShowcase(store.token, newIds)
    ElMessage.success(t("game.showcaseAddSuccess"))
    emit("updated")
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.showcaseError"))
  }
}

async function unshowcaseItem() {
  const instanceId = currentInstanceId()
  if (!store.token || !instanceId || !canUnshowcase.value) return
  try {
    const current = myProfile.value?.showcase || []
    const newIds = current.filter(s => s.instanceId !== instanceId).map(s => s.instanceId)
    await localUserStore.setShowcase(store.token, newIds)
    ElMessage.success(t("game.showcaseRemoveSuccess"))
    emit("updated")
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.showcaseError"))
  }
}
```

**模板传递**：

```vue
<ItemDetailActionBar
  :can-use-skillbook="canShowUseSkillbook"
  :can-use-collectible="canShowUseCollectible"
  :can-enhance="canShowEnhance"
  :can-equip="canShowEquip"
  :can-unequip="canShowUnequip"
  :can-repair="canShowRepair"
  :can-show-affix-operations="canShowAffixOperations"
  :can-detach-affix="canDetachAffixAction"
  :can-attach-affix="canAttachAffixAction"
  :can-enchant-affix="canEnchantAffixAction"
  :can-show-market="canShowMarket"
  :can-show-recycle="canShowRecycle"
  :can-showcase="canShowcase"
  :can-unshowcase="canUnshowcase"
  @use-skillbook="useSkillbook"
  @use-collectible="useCollectible"
  @goto-enhance="gotoEnhance"
  @equip="openEquipDialog"
  @unequip="unequipItem"
  @repair="openRepairDialog"
  @goto-extract="gotoExtract"
  @goto-embed="gotoEmbed"
  @goto-enchant="gotoEnchant"
  @goto-market="gotoMarket"
  @recycle="openRecycleDialog"
  @showcase="showcaseItem"
  @unshowcase="unshowcaseItem"
/>
```

### 6. i18n 国际化

所有 locale 文件添加以下 key：

**`web/src/locales/zh-CN.ts`**：

```typescript
"game.userProfileShowcaseTitle": "展台",
"game.showcaseAdd": "展示",
"game.showcaseRemove": "取消展示",
"game.showcaseAddSuccess": "已添加到展台",
"game.showcaseRemoveSuccess": "已从展台移除",
"game.showcaseFull": "展台已满（最多3个）",
"game.showcaseError": "展台操作失败",
```

**`web/src/locales/zh-TW.ts`**：

```typescript
"game.userProfileShowcaseTitle": "展臺",
"game.showcaseAdd": "展示",
"game.showcaseRemove": "取消展示",
"game.showcaseAddSuccess": "已新增至展臺",
"game.showcaseRemoveSuccess": "已從展臺移除",
"game.showcaseFull": "展臺已滿（最多3個）",
"game.showcaseError": "展臺操作失敗",
```

**`web/src/locales/en.ts`**：

```typescript
"game.userProfileShowcaseTitle": "Showcase",
"game.showcaseAdd": "Showcase",
"game.showcaseRemove": "Remove",
"game.showcaseAddSuccess": "Added to showcase",
"game.showcaseRemoveSuccess": "Removed from showcase",
"game.showcaseFull": "Showcase full (max 3)",
"game.showcaseError": "Showcase operation failed",
```

**`web/src/locales/ko.ts`** 和 **`web/src/locales/ru.ts`** 同理。

## 实施顺序

| 步骤 | 任务 | 文件 |
|------|------|------|
| 1 | DB 迁移 | `server/schema/migrations/20260623_player_showcase.sql` |
| 2 | Store 层 + 错误哨兵 | `server/internal/store/store_showcase.go`, `errors.go` |
| 3 | HTTP handler + 路由注册 | `server/internal/httpapi/server_showcase.go`, `server_http_base.go` |
| 4 | Profile API 集成 | `server/internal/httpapi/server_player_profile.go` |
| 5 | 前端类型 + store action | `web/src/stores/local-user.ts` |
| 6 | ShowcaseCabinet 组件 | `web/src/components/game/item/ShowcaseCabinet.vue` |
| 7 | UserProfileDialogHost 集成 | `web/src/components/common/UserProfileDialogHost.vue` |
| 8 | ItemDetailActionBar 按钮 | `web/src/components/game/item/ItemDetailActionBar.vue` |
| 9 | ItemDetailPanel computed + handlers | `web/src/components/game/item/ItemDetailPanel.vue` |
| 10 | i18n keys | `web/src/locales/*.ts` |
| 11 | 测试 | backend store/handler tests |
| 12 | 验证 | `make harness-pr` |

## 关键注意事项

### Profile 缓存失效

`handleShowcaseSet` 成功后**必须**清除 profile 内存缓存：

```go
s.playerProfileMu.Lock()
delete(s.playerProfileMem, userID)
s.playerProfileMu.Unlock()
```

否则旧展示数据最多 60s 才过期。

### TypeScript 严格

- `PlayerShowcaseItem` 必须完整类型定义
- 禁止 `any` 类型
- 所有 API 响应必须类型化

### i18n 强制

- 所有用户可见文本走 `t('key')`
- 禁止硬编码中文/英文
- 新增 key 必须覆盖所有 locale 文件

### WS 同步

`handleShowcaseSet` 的 `syncAfterMutation` 必须带 `Inventory: true`，确保多设备同步：

```go
s.syncAfterMutation(mutationSyncScope{
    UserIDs:     []int64{userID},
    PlayerState: true,
    Inventory:   true,
})
```

### 错误包装

Store 层错误必须用 `fmt.Errorf("context: %w", err)` 包装，保留错误链：

```go
return nil, fmt.Errorf("showcase get: %w", err)
```

### 懒验证 vs 主动清理

本设计采用**懒验证**策略：
- `GetShowcase` JOIN 时过滤 `state='inventory'`
- 装备中/拍卖中的物品自动不显示
- 物品删除由 `ON DELETE CASCADE` 处理

优点：不需要在 equip/auction 代码中添加清理钩子。
缺点：数据库中可能有短暂的"脏"记录（不影响用户体验）。

## 验证方式

### 后端测试

```bash
cd server && go test ./internal/store/ -run Showcase
cd server && go test ./internal/httpapi/ -run Showcase
```

### 前端检查

```bash
cd web && npm run typecheck
cd web && npm run build
```

### 手动测试

1. 仓库中选择物品 → 点击"展示" → 确认成功提示
2. 查看自己 profile → 确认展示柜显示 3 个物品
3. 查看其他玩家 profile → 确认展示柜正常显示（或无物品时不显示）
4. 点击"取消展示" → 确认物品从展示柜移除
5. 装备已展示物品 → 确认展示柜自动不显示该物品
6. 回收已展示物品 → 确认展示记录自动清理

### 完整 PR 检查

```bash
make harness-pr
```

## 文件清单

### 新增文件

| 文件 | 说明 |
|------|------|
| `server/schema/migrations/20260623_player_showcase.sql` | DB 迁移 |
| `server/internal/store/store_showcase.go` | Store 层 |
| `server/internal/httpapi/server_showcase.go` | HTTP handler |
| `web/src/components/game/item/ShowcaseCabinet.vue` | 展示柜组件 |

### 修改文件

| 文件 | 说明 |
|------|------|
| `server/internal/store/errors.go` | 新增错误哨兵 |
| `server/internal/httpapi/server_http_base.go` | 路由注册 |
| `server/internal/httpapi/server_player_profile.go` | Profile API 集成 showcase |
| `web/src/stores/local-user.ts` | 类型 + setShowcase action |
| `web/src/components/common/UserProfileDialogHost.vue` | 集成 ShowcaseCabinet |
| `web/src/components/game/item/ItemDetailActionBar.vue` | 新增展示/取消展示按钮 |
| `web/src/components/game/item/ItemDetailPanel.vue` | computed + handlers |
| `web/src/locales/*.ts` | i18n keys (zh-CN, zh-TW, en, ko, ru) |
