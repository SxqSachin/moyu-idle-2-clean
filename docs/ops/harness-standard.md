# Harness 标准（全链路）

## 目标
- 统一本项目的验证与质量门禁，覆盖测试、构建、并发回归与发布验收。
- 让本地与 CI 执行行为一致，保证可复现、可阻断、可回滚。

## 分层标准
1. **L0 静态与构建层**
   - Web: `npm run build`
   - shared game-data: `node packages/game-data/scripts/build.mjs`
2. **L1 单元测试层**
   - Server: `go test ./...`（纯逻辑单测）
   - Web: `npm run test:unit`（Vitest）
3. **L2 集成层**
   - 服务端依赖采用 Docker Compose 拉起 Postgres/Redis，按迁移脚本初始化。
4. **L3 端到端层**
   - Web/Electron 前端行为以 Playwright 维护冒烟链路。
5. **L4 并发与事务回归层**
   - 覆盖同玩家串行、跨玩家并行、市场幂等与防超卖等关键链路。

## 工具链标准
- 前端单测与契约：**Vitest**
- 端到端冒烟：**Playwright**
- 服务端单测/并发回归：**Go testing**
- 服务端依赖环境：**Docker Compose(Postgres/Redis)**

## PR 质量门（严格）
PR 必须通过：
1. `node packages/game-data/scripts/build.mjs`
2. `cd server && go mod tidy && go test ./...`
3. `cd web && npm run build`
4. `cd web && npm run test:unit`
5. `cd web && npm run test:e2e`

## 并发与一致性要求
- 调度链路：同玩家事件必须串行执行，跨玩家允许并行。
- 市场链路：必须以数据库事务执行；买入接口必须携带 `X-Idempotency-Key` 并可重复请求去重。
- 锁与重试：事务冲突需要明确错误返回，不做静默吞错。

## i18n 与双端兼容要求
- 默认六语言（`zh-CN`/`zh-TW`/`en-US`/`ja`/`ko`/`ru`）键集保持一致。
- Web 与 Electron 共享前端行为回归用例，避免分叉标准。

## 数据源一致性要求
- 前后端共享配置只允许来自 `packages/game-data`。
- 禁止在 server/web 维护手工双份配置。

## 执行入口
- CI 工作流：`.github/workflows/harness.yml`
- 本地推荐顺序：
  1. `node packages/game-data/scripts/build.mjs`
  2. `cd server && go mod tidy && go test ./...`
  3. `cd web && npm ci && npm run build && npm run test:unit && npm run test:e2e`
