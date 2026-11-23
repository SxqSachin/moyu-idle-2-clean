# Claude Code Project Instructions

> 本文件为 Claude Code 自动加载的项目级指令。

## 强制规范

**编码前必读**：`docs/ops/harness-coding-standard.md`

### 核心原则（不可妥协）
1. **单一数据源**：游戏配置只允许 `packages/game-data/source/`，禁止手工双份
2. **可运行/可验证/可回滚**：所有变更必须满足
3. **错误显式传播**：禁止静默吞错，使用 `fmt.Errorf("context: %w", err)`
4. **并发安全**：市场/交易/库存必须数据库事务 + 明确锁策略
5. **i18n 强制**：所有用户可见文本用 i18n，禁止硬编码中文

### 代码风格速查

**Go 后端：**
- 结构化日志：`logger.InfoContext(ctx, "event.name", "key", value)`
- 错误包装：`return fmt.Errorf("operation: %w", err)`
- 超时控制：`context.WithTimeout(ctx, 5*time.Second)`
- 禁止裸 goroutine，必须有生命周期管理

**Vue 3 前端：**
- Composition API + `<script setup lang="ts">`
- Pinia 状态管理，禁止全局变量
- 所有文本 `t('key')`，禁止硬编码
- TypeScript 严格模式，禁止 `any`

### 测试要求

变更涉及以下必须添加测试：
- ✅ 新函数/方法 → 单元测试
- ✅ 市场/交易 → 并发测试
- ✅ 关键 UI 路径 → E2E 测试
- ✅ 数据库变更 → 迁移脚本 + 回滚脚本

### 验证流程

```bash
# PR 前必跑
make harness-pr

# 或分步
node packages/game-data/scripts/build.mjs
cd server && go mod tidy && go test ./...
cd web && npm run typecheck && npm run build && npm run test:unit
```

### 禁止行为

- ❌ `TODO: fix later` 占位符
- ❌ TypeScript `any` 类型
- ❌ 硬编码配置值
- ❌ 跳过错误处理
- ❌ 无测试的新功能
- ❌ 手工双份配置

## 文档入口

- 总览：`docs/index.md`
- 前端：`docs/frontend/README.md`
- 后端：`docs/backend/README.md`
- 编码规范详解：`docs/ops/harness-coding-standard.md`

## Git 规范

**分支命名**：`feature/xxx`, `fix/xxx`, `refactor/xxx`, `docs/xxx`, `test/xxx`

**提交信息**（Conventional Commits）：
```
feat(market): add batch buy API

- Implement POST /api/market/batch-buy
- Require X-Idempotency-Key
- Add concurrent tests

Closes #123
```

## 安全检查清单

生成代码前确认：
- [ ] 受保护路由有 `authMiddleware`
- [ ] 输入有边界检查
- [ ] 无敏感数据入日志
- [ ] 金融操作有幂等键
- [ ] 数据库查询走索引

## 性能检查清单

- [ ] 列表接口有分页
- [ ] Redis 缓存有 TTL + 失效策略
- [ ] 无 N+1 查询
- [ ] 大事务拆小事务

---

**本标准强制生效，违反将阻断 PR。**
