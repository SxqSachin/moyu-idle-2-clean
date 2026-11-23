# 前端开发指南（Web）

## 1. 目录与别名约定

前端代码根目录为 `web/src`，统一使用以下 import 别名，避免多层 `../../`：

| 别名 | 映射目录 |
|---|---|
| `@` | `web/src` |
| `@components` | `web/src/components` |
| `@services` | `web/src/services` |
| `@stores` | `web/src/stores` |
| `@views` | `web/src/views` |
| `@utils` | `web/src/utils` |
| `@composables` | `web/src/composables` |
| `@game-data` | `packages/game-data/generated` |

> 别名配置来源：`web/vite.config.ts`、`web/vitest.config.ts`、`web/tsconfig.json`。新增别名时需三处同步。

## 2. Import 写法示例

```ts
import { apiGet } from "@services/api"
import { useGameStore } from "@stores/game"
import GameItem from "@components/game/GameItem.vue"
import actionsChunk from "@game-data/chunks/actions.json"
```

## 3. 开发约束

1. 新增代码优先使用别名导入，不再引入多级相对路径。
2. 跨目录公共逻辑优先归类到 `services`、`utils`、`composables`，并通过别名引用。
3. 变更 alias 后，需确保 `build` 与 `test:unit` 通过，再提交。
4. 类型定义与方法实现分离：共享业务类型放 `web/src/types/*`，服务函数放 `web/src/services/*`。
5. 物品类型以 `packages/game-data/generated/game-data.ts` 为准，前端仅在 `web/src/types/game-data.ts` 做类型别名，不在服务层重复定义字段。
6. 装备判定统一走 `item-definition`：通过 `item.isEquipment === true`（或 `asEquipmentItemDefinition`）做安全收窄，不在页面层手写类型判断分支。
7. 组件单一职责，通过多个组件组合形成完整功能
8. 组件用element-plus，css用tailwindcss，前端逻辑可以用composable来组合归并
9. Tailwind `important` 语法统一使用后缀写法（如 `my-2!`、`p-0!`），不要混用旧前缀写法（`!my-2`）。
10. Tailwind v4 在 Vue `<style scoped>` 中使用 `@apply` 时，必须先添加 `@reference` 指向共享样式入口；推荐使用 package import 简写（`@reference "#app-style";`，由 `web/package.json#imports` 映射到 `web/src/style.css`），避免多层 `../../`。
11. 游戏配置读取优先使用 `@game-data/chunks/*.json` 按域导入，避免默认整包引入 `@game-data/game-data.json`；确需整包时统一走 `web/src/services/game-data-catalog.ts`。构建策略上 `chunks/*.json` 默认独立分包，极小体积配置允许聚合到 `game-data-small` 以减少请求数量。
12. Element Plus 组件默认使用 `unplugin-vue-components` 自动按需引入，不在 `main.ts` 中使用 `app.use(ElementPlus)` 或全量引入 `element-plus/dist/index.css`。
13. 客户端版本刷新通知统一使用 WS 事件 `system.client.update`（`releaseId/kind/recommendedDelaySec/forceAtMs`），由 `views/game/shell/index.vue` 统一处理；前端不再定时轮询 version 接口，初始化只保留 bootstrap 快照。
14. 进入游戏初始化优先走单一 bootstrap 接口（`/api/bootstrap/init`）；该接口可返回压缩 envelope，前端通过 `apiGet` 自动解码，不再为首屏并发触发多条初始化 GET。
15. 图标路径解析仅使用 `core.json.icons` 显式映射；未映射或映射为空路径时必须直接走 fallback，不允许猜测 `/assets/icons/*.png` 路径并发起请求。
16. 分包策略：`/game/*` 主体页面默认静态导入（避免运行时再拉取业务 chunk）；第三方依赖在 `vite.config.ts` 中按构建产物体量做聚合分包，禁止回到组件级/库级过度碎片化拆包。
17. i18n 语言包默认全部走懒加载分包（包括 `zh-CN` 与 `en-US`），启动阶段先加载 fallback 语言，再加载当前语言，避免默认语言被回卷进主包。
