# Harness Coding Standard（全链路编码规范）

> 本文档定义 Claude Code / AI 辅助编码强制标准。适用于本项目所有模块。
> 参考：Codex/OpenAI 最佳实践 + 业界编码规范 + 本项目特定约束。

---

## 1. 核心原则

### 1.1 工程底线（不可妥协）
- **可运行**：代码必须可编译、可执行、可测试
- **可验证**：变更必须有验证手段（测试/日志/监控）
- **可回滚**：数据库变更必须可逆，配置变更可降级
- **可观测**：关键路径必须有日志、指标、追踪点

### 1.2 系统思维（先设计后编码）
- 明确系统边界、数据流、并发模型
- 识别一致性热点（市场/交易/库存）
- 定义幂等边界和重试策略
- 规划错误传播和降级路径

### 1.3 单一数据源（强制）
- 游戏配置：**只允许** `packages/game-data/source/`
- 全局运行参数：**只允许** `packages/game-config/source/`
- 禁止在 server/web 手工维护双份配置

---

## 2. 代码风格规范

### 2.1 Go 后端（server/）

#### 命名约定
```go
// ✅ 正确
func (s *Server) handleMarketBuy(c *fiber.Ctx) error { ... }
type MarketService struct { ... }
var ErrInsufficientBalance = errors.New("insufficient balance")

// ❌ 错误
func (s *Server) Handle_Market_Buy(...) // 不要下划线
func (s *Server) buy(...) // 太模糊
```

#### 错误处理
```go
// ✅ 正确：明确错误传播
if err := s.db.Transaction(ctx, func(tx *sql.Tx) error {
    if err := s.deductBalance(tx, userID, amount); err != nil {
        return fmt.Errorf("deduct balance: %w", err)
    }
    if err := s.createOrder(tx, order); err != nil {
        return fmt.Errorf("create order: %w", err)
    }
    return nil
}); err != nil {
    return httpError{Code: "TRANSACTION_FAILED", Err: err}
}

// ❌ 错误：静默吞错
if err := s.deductBalance(...); err != nil {
    log.Printf("failed: %v", err) // 不要只打印
}
```

#### 并发模式
```go
// ✅ 正确：明确锁策略和超时
func (s *MarketService) Buy(ctx context.Context, req BuyRequest) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    return s.db.Transaction(ctx, func(tx *sql.Tx) error {
        // 行锁：SELECT ... FOR UPDATE
        item, err := s.lockItem(tx, req.ItemID)
        if err != nil {
            return fmt.Errorf("lock item: %w", err)
        }
        // 业务逻辑...
    })
}

// ❌ 错误：无超时、无锁策略
go func() {
    s.processOrder(...) // 不要裸 goroutine
}()
```

#### 日志标准
```go
// ✅ 正确：结构化日志
s.logger.InfoContext(ctx, "market.buy.success",
    "user_id", userID,
    "item_id", itemID,
    "price", price,
    "duration_ms", duration.Milliseconds(),
)

// ❌ 错误：字符串拼接
log.Printf("user %s bought item %s", userID, itemID)
```

### 2.2 Vue 3 前端（web/）

#### 组件结构
```vue
<!-- ✅ 正确：Composition API + <script setup> -->
<template>
  <div class="market-list">
    <el-table :data="listings" v-loading="loading">
      <!-- ... -->
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMarketStore } from '@stores/market'

// Props/Emits 定义
const props = defineProps<{
  categoryId: string
}>()

const emit = defineEmits<{
  (e: 'select', item: MarketItem): void
}>()

// 响应式状态
const loading = ref(false)
const listings = ref<MarketItem[]>([])

// 计算属性
const filteredListings = computed(() => 
  listings.value.filter(item => item.categoryId === props.categoryId)
)

// 生命周期
onMounted(async () => {
  await fetchListings()
})

// 方法
async function fetchListings() {
  loading.value = true
  try {
    listings.value = await useMarketStore().fetchListings()
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.market-list {
  /* 样式 */
}
</style>
```

#### 状态管理
```typescript
// ✅ 正确：Pinia store + 明确 action
export const useMarketStore = defineStore('market', () => {
  const listings = ref<MarketItem[]>([])
  const loading = ref(false)

  async function fetchListings() {
    if (loading.value) return // 防止重复请求
    loading.value = true
    try {
      const res = await api.get<MarketItem[]>('/api/market/listings')
      listings.value = res.data
      return res.data
    } finally {
      loading.value = false
    }
  }

  return { listings, loading, fetchListings }
})
```

#### i18n 强制
```typescript
// ✅ 正确：所有用户可见文本用 i18n
const { t } = useI18n()
const buttonText = t('market.buy') // "买入" / "Buy"

// ❌ 错误：硬编码中文
const buttonText = '买入'
```

#### 游戏表单规范

所有游戏操作表单（创建/编辑/配置 dialog、表单页）遵循统一结构，降低玩家学习成本：

**结构三段式：**
1. **Header**：icon + title + subtitle（可选）
2. **Body**：`<el-form label-position="top">` + grid 布局
3. **Footer**：主操作按钮右对齐

**字段密度规则：**
- 1–4 字段：单列
- 5–10 字段：两列 grid (`grid-cols-1 md:grid-cols-2`)
- 11+ 字段：用 `<fieldset>` + `<legend>` 分组

**强制规则：**
- 必须用 `<el-form>` + `<el-form-item>`，禁止手搓 flex 行替代
- 数值输入用 `el-input-number`；快捷运算按钮（±/÷2/x2）作为 addon，不替代 input
- 状态/稀有度/货币 颜色用 `--game-*` token，禁止硬编码 hex
- 所有 label/placeholder/error 走 `t(...)`

**示例：**
```vue
<el-dialog>
  <div class="game-form">
    <!-- Header -->
    <div class="game-form__header">
      <span class="game-form__icon">🏰</span>
      <h3>{{ t('game.createRoom') }}</h3>
      <p class="game-form__subtitle">{{ t('game.createRoomHint') }}</p>
    </div>

    <!-- Body -->
    <el-form :model="form" label-position="top">
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <el-form-item :label="t('game.roomName')">
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item :label="t('game.difficulty')">
          <el-select v-model="form.difficulty">...</el-select>
        </el-form-item>
      </div>
    </el-form>

    <!-- Footer -->
    <div class="game-form__footer">
      <el-button @click="cancel">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" @click="submit">{{ t('common.create') }}</el-button>
    </div>
  </div>
</el-dialog>
```

#### 主题系统

游戏 UI 支持亮度（light/dark）× 皮肤（default/mystic/ember）组合主题。

**Token 命名空间：**
- `--app-*`：基础 UI 色（bg, surface, border, text, primary）
- `--game-*`：游戏语义色（gold, xp, hp, success, danger, rarity-*）
- `--el-*`：Element Plus 映射（自动跟随 `--app-*`，禁止直接覆盖）

**亮度 vs 皮肤：**
- **亮度**：`data-theme="light|dark"`（通过 `.dark` class），后端同步（已有 API）
- **皮肤**：`data-skin="default|mystic|ember"`，客户端 localStorage（`player-skin` key）

**新增皮肤步骤：**
1. 在 `web/src/themes/skins.css` 添加 `html[data-skin="my-skin"]` 块，覆盖 `--app-primary/hover/active` 和 `--game-*` 色 token
2. 在 `web/src/themes/registry.ts` 的 `SKINS` 数组添加 ID
3. 在 `web/src/locales/*.ts` 添加 `skin.mySkin` i18n key
4. 在 `web/src/themes/registry.ts` 的 `SKIN_SWATCHES` 添加预览色

**禁止行为：**
- ❌ 组件内硬编码颜色（如 `color: #7c3aed`）
- ❌ 直接覆盖 `--el-*`（应覆盖 `--app-*` 让映射自动跟随）
- ❌ 皮肤覆盖表面/亮度 token（`--app-bg/surface/text` — 这些由 `.dark` 控制）

### 2.3 TypeScript/JavaScript 工具

#### 类型安全
```typescript
// ✅ 正确：明确类型
interface MarketListing {
  id: string
  itemId: string
  price: number
  quantity: number
  sellerId: string
  createdAt: string // ISO 8601
}

function formatPrice(price: number): string {
  return price.toLocaleString('en-US')
}

// ❌ 错误：any 类型
function formatPrice(price: any): any { ... }
```

#### 异步错误处理
```typescript
// ✅ 正确：try-catch + 用户友好错误
async function buyItem(listingId: string) {
  try {
    await api.post('/api/market/buy', { listingId })
    ElMessage.success(t('market.buySuccess'))
  } catch (err) {
    const msg = err instanceof Error ? err.message : t('error.unknown')
    ElMessage.error(msg)
    logger.error('buyItem.failed', { listingId, err })
  }
}
```

---

## 3. 测试规范

### 3.1 测试金字塔（强制）

```
        ╱╲
       ╱E2E╲        L3: Playwright 端到端（关键路径）
      ╱──────╲
     ╱Integration╲  L2: Docker Compose 集成（数据库/Redis）
    ╱──────────────╲
   ╱  Unit Tests    ╲ L1: Vitest/Go testing（纯逻辑）
  ╱──────────────────╲
 ╱  Static Analysis   ╲ L0: Build/Typecheck/Lint
╱──────────────────────╲
```

### 3.2 单元测试（L1）

#### Go 后端
```go
// ✅ 正确：表驱动测试
func TestMarketService_Buy(t *testing.T) {
    tests := []struct {
        name    string
        setup   func(t *testing.T) *MarketService
        req     BuyRequest
        wantErr bool
        errCode string
    }{
        {
            name: "success",
            setup: func(t *testing.T) *MarketService {
                return newTestMarketService(t, withBalance(1000))
            },
            req:     BuyRequest{ItemID: "item-1", Price: 100},
            wantErr: false,
        },
        {
            name: "insufficient balance",
            setup: func(t *testing.T) *MarketService {
                return newTestMarketService(t, withBalance(50))
            },
            req:     BuyRequest{ItemID: "item-1", Price: 100},
            wantErr: true,
            errCode: "INSUFFICIENT_BALANCE",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            svc := tt.setup(t)
            err := svc.Buy(context.Background(), tt.req)
            if (err != nil) != tt.wantErr {
                t.Errorf("Buy() error = %v, wantErr %v", err, tt.wantErr)
            }
            if tt.wantErr && errCode(err) != tt.errCode {
                t.Errorf("Buy() errCode = %v, want %v", errCode(err), tt.errCode)
            }
        })
    }
}
```

#### TypeScript 前端
```typescript
// ✅ 正确：Vitest 单测
import { describe, it, expect, vi } from 'vitest'
import { useMarketStore } from '@stores/market'

describe('useMarketStore', () => {
  it('should fetch listings', async () => {
    const mockListings = [
      { id: '1', itemId: 'item-1', price: 100 },
      { id: '2', itemId: 'item-2', price: 200 },
    ]
    
    vi.spyOn(api, 'get').mockResolvedValue({ data: mockListings })
    
    const store = useMarketStore()
    await store.fetchListings()
    
    expect(store.listings).toEqual(mockListings)
    expect(store.loading).toBe(false)
  })

  it('should prevent duplicate requests', async () => {
    const store = useMarketStore()
    const promise1 = store.fetchListings()
    const promise2 = store.fetchListings() // 第二次应该被跳过
    
    await Promise.all([promise1, promise2])
    expect(api.get).toHaveBeenCalledTimes(1)
  })
})
```

### 3.3 端到端测试（L3）

```typescript
// ✅ 正确：Playwright E2E
import { test, expect } from '@playwright/test'

test.describe('Market Buy Flow', () => {
  test('should complete purchase with idempotency', async ({ page }) => {
    // 登录
    await page.goto('/auth')
    await page.fill('[data-testid="username"]', 'testuser')
    await page.fill('[data-testid="password"]', 'password123')
    await page.click('[data-testid="login-button"]')
    
    // 进入市场
    await page.goto('/game/market')
    await page.waitForSelector('[data-testid="market-list"]')
    
    // 购买商品
    const listing = page.locator('[data-testid="listing-item"]').first()
    await listing.click()
    await page.click('[data-testid="buy-button"]')
    
    // 验证成功
    await expect(page.locator('[data-testid="success-message"]')).toBeVisible()
    
    // 验证库存更新
    const inventory = await page.locator('[data-testid="inventory-count"]').textContent()
    expect(Number(inventory)).toBeGreaterThan(0)
  })
})
```

### 3.4 并发测试（L4 - 后端关键链路）

```go
// ✅ 正确：并发安全测试
func TestMarketService_ConcurrentBuy(t *testing.T) {
    svc := newTestMarketService(t, 
        withBalance(1000),
        withItem("item-1", 10), // 10 个库存
    )
    
    var wg sync.WaitGroup
    errors := make(chan error, 100)
    
    // 50 个并发购买请求
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            err := svc.Buy(context.Background(), BuyRequest{
                ItemID: "item-1",
                Price:  10,
            })
            if err != nil {
                errors <- err
            }
        }()
    }
    
    wg.Wait()
    close(errors)
    
    // 验证：最多 10 个成功（库存限制）
    successCount := 50 - len(errors)
    if successCount > 10 {
        t.Errorf("expected at most 10 success, got %d", successCount)
    }
}
```

---

## 4. 文档规范

### 4.1 代码注释（最小化原则）

```go
// ✅ 正确：解释 WHY，不是 WHAT
// 使用 Redis 分布式锁防止超卖，因为数据库行锁在高并发下性能不足。
func (s *MarketService) lockInventory(ctx context.Context, itemID string) (UnlockFunc, error) {
    // ...
}

// ❌ 错误：显而易见的注释
// 获取商品列表
func (s *MarketService) GetListings() []Listing { ... }
```

### 4.2 API 文档（强制）

```go
// ✅ 正确：OpenAPI 注释
// BuyMarketItem godoc
// @Summary      购买市场商品
// @Description  使用金币购买市场挂单商品，需要携带幂等键防止重复扣款
// @Tags         market
// @Accept       json
// @Produce      json
// @Param        X-Idempotency-Key  header    string  true  "幂等键（UUID）"
// @Param        request           body      BuyRequest  true  "购买请求"
// @Success      200  {object}  Response{data=BuyResponse}
// @Failure      400  {object}  Response{error=ErrorResponse}
// @Failure      409  {object}  Response{error=ErrorResponse}  "余额不足/库存不足"
// @Router       /api/market/buy [post]
// @Security     BearerAuth
func (s *Server) handleMarketBuy(c *fiber.Ctx) error { ... }
```

### 4.3 变更日志（PR 强制）

```markdown
## 变更类型
- [ ] 新功能（feature）
- [x] Bug 修复（fix）
- [ ] 重构（refactor）
- [ ] 文档（docs）
- [ ] 测试（test）

## 影响范围
- [ ] 前端（web/）
- [x] 后端（server/）
- [ ] 共享配置（packages/game-data/）
- [ ] 基础设施（infra/）

## 破坏性变更
- [ ] 无
- [ ] 有（请说明）

## 测试验证
- [x] 单元测试通过
- [x] 并发测试通过（如涉及市场/交易）
- [ ] E2E 测试通过（如涉及前端交互）

## 文档更新
- [ ] 已更新 API 文档
- [ ] 已更新架构文档（如涉及）
```

---

## 5. 安全规范

### 5.1 认证与授权（强制）

```go
// ✅ 正确：JWT 中间件保护
func (s *Server) routes() {
    // 公开路由
    public := s.Group("/api")
    public.Post("/register", s.handleRegister)
    public.Post("/login", s.handleLogin)
    
    // 受保护路由
    protected := s.Group("/api", s.authMiddleware())
    protected.Post("/market/buy", s.replayGuardMiddleware(), s.handleMarketBuy)
    protected.Get("/inventory/instances", s.handleGetInventory)
}

// ❌ 错误：遗漏鉴权
s.Post("/api/admin/reset", s.handleAdminReset) // 缺少 adminMiddleware
```

### 5.2 输入验证（强制）

```go
// ✅ 正确：边界检查
func (s *Server) handleMarketList(c *fiber.Ctx) error {
    var req ListRequest
    if err := c.BodyParser(&req); err != nil {
        return httpError{Code: "INVALID_REQUEST", Err: err}
    }
    
    // 价格边界
    if req.Price < 1 || req.Price > 1_000_000 {
        return httpError{Code: "PRICE_OUT_OF_RANGE", Message: "price must be 1-1000000"}
    }
    
    // 数量边界
    if req.Quantity < 1 || req.Quantity > 1000 {
        return httpError{Code: "QUANTITY_OUT_OF_RANGE", Message: "quantity must be 1-1000"}
    }
    
    // ...
}
```

### 5.3 敏感数据（强制）

```go
// ✅ 正确：不记录敏感信息
s.logger.InfoContext(ctx, "user.login.success",
    "user_id", userID,
    "ip", clientIP,
)

// ❌ 错误：记录密码/token
s.logger.InfoContext(ctx, "user.login",
    "password", password, // 绝对禁止
    "token", token,
)
```

---

## 6. Git 与协作规范

### 6.1 分支命名

```bash
# ✅ 正确
feature/market-batch-buy
fix/inventory-duplicate-items
refactor/market-service-extract
docs/update-api-reference
test/add-concurrent-tests

# ❌ 错误
my-feature
wip
fix-bug
```

### 6.2 提交信息（Conventional Commits）

```bash
# ✅ 正确
feat(market): add batch buy API with idempotency support

- Implement POST /api/market/batch-buy endpoint
- Require X-Idempotency-Key for replay protection
- Add transaction-level locking for inventory
- Include unit tests for concurrent scenarios

Closes #123

# ❌ 错误
update code
fix bug
wip
```

### 6.3 PR 描述（强制模板）

```markdown
## 概述
简要描述本次变更的目的和背景。

## 变更内容
- 详细列出主要变更
- 说明技术选型和实现思路

## 测试验证
- [ ] 本地运行 `make harness-pr` 通过
- [ ] 涉及市场/交易的变更已添加并发测试
- [ ] 前端变更已更新 E2E 测试

## 影响评估
- 性能影响：是/否（说明）
- 数据库变更：是/否（迁移脚本）
- 配置变更：是/否（说明）

## 截图/录屏（如涉及 UI）
[附件]

## 相关链接
- Issue: #123
- 设计文档: docs/xxx.md
```

---

## 7. 性能规范

### 7.1 数据库查询（强制）

```go
// ✅ 正确：索引友好 + 分页
func (s *MarketStore) ListListings(ctx context.Context, req ListRequest) ([]Listing, error) {
    query := `
        SELECT id, item_id, price, quantity, seller_id, created_at
        FROM market_listings
        WHERE status = 'active'
          AND price >= $1
          AND price <= $2
        ORDER BY created_at DESC
        LIMIT $3 OFFSET $4
    `
    // 确保 (status, price, created_at) 有复合索引
    rows, err := s.db.QueryContext(ctx, query, req.MinPrice, req.MaxPrice, req.Limit, req.Offset)
    // ...
}

// ❌ 错误：全表扫描
rows, err := s.db.QueryContext(ctx, "SELECT * FROM market_listings")
```

### 7.2 缓存策略（Redis）

```go
// ✅ 正确：缓存失效 + TTL
func (s *InventoryService) GetItems(ctx context.Context, userID string) ([]Item, error) {
    cacheKey := fmt.Sprintf("inventory:%s", userID)
    
    // 尝试从缓存读取
    cached, err := s.redis.Get(ctx, cacheKey).Result()
    if err == nil {
        var items []Item
        if err := json.Unmarshal([]byte(cached), &items); err == nil {
            return items, nil
        }
    }
    
    // 缓存未命中，查询数据库
    items, err := s.store.GetItems(ctx, userID)
    if err != nil {
        return nil, err
    }
    
    // 写入缓存（30s TTL）
    data, _ := json.Marshal(items)
    s.redis.Set(ctx, cacheKey, data, 30*time.Second)
    
    return items, nil
}

// 写路径必须失效缓存
func (s *InventoryService) AddItem(ctx context.Context, userID string, item Item) error {
    if err := s.store.AddItem(ctx, userID, item); err != nil {
        return err
    }
    // 失效缓存
    s.redis.Del(ctx, fmt.Sprintf("inventory:%s", userID))
    return nil
}
```

---

## 8. Claude Code 集成规范

### 8.1 代码生成检查清单

Claude Code 生成代码前必须确认：

- [ ] 是否符合单一数据源原则？
- [ ] 是否添加了必要的错误处理？
- [ ] 是否考虑了并发安全？
- [ ] 是否遵循命名约定？
- [ ] 是否需要添加测试？
- [ ] 是否需要更新文档？
- [ ] 是否涉及敏感数据处理？
- [ ] 是否有性能隐患？

### 8.2 自动验证流程

```bash
# Claude Code 应在关键变更后建议运行：
make harness-pr

# 或分步验证：
node packages/game-data/scripts/build.mjs
cd server && go mod tidy && go test ./...
cd web && npm run typecheck && npm run build && npm run test:unit
```

### 8.3 禁止行为

- ❌ 不要生成 `TODO: fix later` 占位符
- ❌ 不要使用 `any` 类型（TypeScript）
- ❌ 不要硬编码配置值
- ❌ 不要跳过错误处理
- ❌ 不要生成无测试的新功能
- ❌ 不要创建手工双份配置

---

## 9. 参考资源

### 9.1 项目文档入口
- 总览：`docs/index.md`
- 前端：`docs/frontend/README.md`
- 后端：`docs/backend/README.md`
- 部署：`docs/deployment/native-split-deployment.md`

### 9.2 外部参考
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Vue 3 Style Guide](https://vuejs.org/style-guide/)
- [TypeScript Deep Dive](https://basarat.gitbook.io/typescript/)
- [OWASP Top 10](https://owasp.org/www-project-top-ten/)

---

## 10. 版本与更新

- **版本**：1.0.0
- **最后更新**：2026-06-11
- **维护者**：项目团队
- **更新频率**：按需更新，重大变更需团队评审

---

**本标准为强制规范，所有 PR 必须遵守。Claude Code 应在生成代码时自动遵循。**
