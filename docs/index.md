# 文档索引（AI 优先）

只读本文件即可决定下一步，不需要先读子目录 README。

## 默认最小读取集
- `docs/index.md`（本文件）
- `docs/catalog/ai-development-guidelines.md`（AI 协作约束）

## 任务 -> 直达文档
| 任务 | 直接读这些文档 |
|---|---|
| 本地启动 / 联调 | `README.md` |
| 线上部署 / 发布 | `docs/deployment/native-split-deployment.md`, `docs/ops/infra-sizing-and-scale.md` |
| 多实例 / WS / 并发架构 | `docs/architecture/scale-architecture.md` |
| 战斗房间运行时 / 回放时序 | `docs/backend/battle-room-runtime.md` |
| 技能脚本开发（复杂选目标/自定义结算） | `docs/backend/skill-script-development.md`, `docs/planning/excel-data-rules.md` |
| 动作队列 / 调度 / 离线快进 | `docs/backend/action-queue-design.md`, `docs/architecture/scheduler-design.md` |
| 封号流程 / 封号副作用 / 后台封号接口 | `docs/backend/user-ban-runtime.md`, `docs/deployment/native-split-deployment.md` |
| 市场交易 / 幂等 / 事务一致性 | `docs/backend/market-transaction.md`, `docs/backend/inventory-concurrency-and-anti-abuse.md` |
| 后端代码结构梳理 / 目录重构 | `docs/backend/backend-code-overview.md`, `docs/backend/battle-room-runtime.md` |
| 数据表与迁移影响 | `docs/backend/database-schema.md` |
| 共享配置（game-data） | `docs/backend/shared-game-data.md` |
| 前端工程约定 | `docs/frontend/frontend-development.md` |
| 装备/属性页面与数据语义 | `docs/frontend/equipment-and-stats.md` |
| 炼金系统（转换/炼金台/点金/熔炉）| `docs/alchemy-design.md` |
| UI 规范 | `docs/frontend/ui-design.md`, `docs/frontend/gameplay-ui-standard.md` |
| 策划数据与 i18n | `docs/planning/excel-data-rules.md`, `docs/planning/game-i18n-logic.md`, `docs/planning/ai-i18n-generation-guide.md` |

## 目录（仅路径）
- `docs/architecture/`
- `docs/backend/`
- `docs/frontend/`
- `docs/deployment/`
- `docs/ops/`
- `docs/planning/`
- `docs/catalog/`

## 读取规则
1. 先看“任务 -> 直达文档”，只读命中的文档。
2. 单次任务尽量控制在 1~3 个文档。
3. 未命中时再搜索，不先通读全量 docs。

## 文档治理规则（防膨胀）
1. 新增文档前，先判断能否并入已有文档；能并入就不新建。
2. 避免同一约束在多个文档重复定义；重复内容改为单点定义 + 链接。
3. 长文档必须有 TL;DR 与“何时阅读”段落，保证首屏可决策。
