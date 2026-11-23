# deep-article 前端 UI/UX 全量迁移规范（执行版）

## 1. 目标与边界

- **目标**：将 MySecondWebGame 前端玩法体验迁移为 deep-article 同等级别的菜单体系、页面结构、交互路径、视觉层级与全局 UI 行为。
- **边界**：
  - 本阶段仅迁移前端 UI/UX，不改后端协议设计。
  - 后端未就绪能力统一采用：**保留完整界面 + 禁用提交按钮 + 标注“后端待接入”**。
  - 迁移时必须同步补齐 `zh-CN` + `en-US` i18n key，不允许“后补翻译”。

## 2. 参考源（Source of Truth）

以 deep-article 下列文件作为迁移基准：

| 文件 | 用途 |
|---|---|
| `pages/index.vue` | 主壳层、菜单生成、tab-query 联动、桌面/移动菜单、全局浮层 |
| `components/Game/layout/HomePage.vue` | 首页聚合布局 |
| `components/Game/layout/GameActionPanel.vue` | 动作面板分组/Tab 交互 |
| `composables/game/useDarkMode.ts` | 深色模式切换策略 |
| `shared/data/allGameActions.ts` | 动作主类 `allGameActionMainType`（动态菜单驱动源） |

## 3. 目标信息架构（必须按此顺序迁移）

### 3.1 顶层壳层结构

1. 顶部栏（Logo/主题切换/用户下拉）
2. 左侧桌面菜单（分组 + 子菜单 + 动态菜单）
3. 移动端抽屉菜单（分区块平铺）
4. 内容主区域（按 `tab` 选择渲染）
5. 全局浮层（断线提示、聊天面板等）

### 3.2 菜单结构（deep-article 对齐）

按顺序定义，不得跳序：

1. `homepage`
2. 分割线
3. `character` / `friend` / `quest`
4. 分割线
5. 子菜单 `myhome`：`inventory` / `farmingPlant` / `fishing` / `cattery` / `tavern` / `building`
6. 子菜单 `village`：`village` / `shop` / `market` / `auction` / `playerShop`
7. `guild`
8. 分割线
9. `battle` / `expedition`
10. 分割线
11. `enhance` / `dismantle` / `mysterious`
12. 分割线
13. **动态动作菜单**（来源：`allGameActionMainType`，排除被自定义渲染接管的主类）
14. `rankingList` / `cheaterList`
15. 分割线
16. `gameguide` / `dictionary` / `wiki` / `instructions` / `updateLog` / `report` / `gm` / `sponsor`
17. 用户下拉菜单页：`selfProfile` / `bindEmail` / `resetPassword` / `switchAccount`

## 4. 菜单驱动层（必须先落地）

### 4.1 Registry 数据结构

在 MySecondWebGame 建立统一 registry，不再手写散落菜单：

```ts
type MenuNode = {
  type: 'item' | 'submenu' | 'divider'
  index?: string
  titleKey?: string
  icon?: string
  class?: string
  badge?: boolean
  visible?: () => boolean
  onClick?: (ctx: MenuContext) => void
  children?: MenuNode[]
  source?: 'static' | 'dynamic-action' | 'user-dropdown'
}
```

### 4.2 动态菜单解析器（Dynamic Resolver）

- 输入：动作主类配置（可由 `packages/game-data` 或 API 提供）。
- 输出：`type: item` 菜单数组，插入到固定锚点（`mysterious` 之后、`rankingList` 之前）。
- 规则：
  - 支持过滤（隐藏/未解锁/被 custom renderer 接管）。
  - 保证稳定排序（配置顺序优先）。
  - 每项生成唯一 `index`（如 `action-{id}`）。

## 5. 路由与状态同步规范

### 5.1 目标交互

- 统一以 `tab` 查询参数驱动内容选择（deep-article 同步策略）。
- 对无效 `tab` 自动回退到默认页 `homepage`。
- 菜单选择、用户下拉跳转、事件跳转都通过同一 `updateRouteTab()` 入口。

### 5.2 兼容策略（保留旧路由）

- 当前 `/game/*` 子路由暂时保留，新增兼容映射层：
  - 旧路径进入时转换为 `tab` 状态并渲染对应面板；
  - 新壳层仍可读写历史路径，避免刷新或分享链接失效。

## 6. 页面渲染与交互契约

### 6.1 内容区渲染规则

- 使用 `currentSelectMenuIndex` 单点决定渲染面板。
- 每个面板必须具备：
  - 标题 + 简述（i18n）
  - 主操作区
  - 空态/错误态/加载态
  - 后端待接入态（如适用）

### 6.2 后端未就绪能力（强制）

- UI 结构保持完整可见。
- 所有提交型按钮设为禁用，并附说明文案：
  - `zh-CN`: `后端待接入`
  - `en-US`: `Backend integration pending`
- 禁止伪造成功态；禁止假提交。

### 6.3 事件跳转

保留 deep-article 类似的跨模块导航能力：
- 仓库项 -> 市场筛选
- 仓库项 -> 强化面板
- 仓库项 -> 分解面板
- 统一事件总线 payload（`itemId`/`targetTab`）格式

## 7. 全局层规范

迁移目标包含以下全局 UI 层：

1. 顶栏主题切换（含持久化）
2. 用户下拉及账号相关面板入口
3. Socket 断线提示条
4. 聊天入口按钮与浮动面板（若后端未就绪则展示禁用态）
5. 移动端全屏菜单抽屉

## 8. i18n 规范（迁移即完成）

### 8.1 key 命名建议

- 菜单：`game.menu.{index}`
- 分组：`game.menuGroup.{group}`
- 用户菜单：`game.userMenu.{entry}`
- 占位态：`game.pendingBackend`

### 8.2 执行要求

- 每次迁移一个菜单模块，必须同次提交：
  - 面板组件代码
  - `zh-CN.ts` 新 key
  - `en-US.ts` 对应 key

## 9. MySecondWebGame 对照映射（第一版）

| 当前页面 | deep-article 对应 | 迁移策略 |
|---|---|---|
| `/game/home` | `homepage` | 并入 tab 面板，保留 queue 卡片并升级布局 |
| `/game/market` | `market` | 对齐市场面板结构，支持参数筛选联动 |
| `/game/combat` | `battle` | 对齐战斗入口与子分区布局 |
| `/game/attributes` | `character` | 角色信息/状态面板化 |
| `/game/training` `/game/work` `/game/exploration` `/game/crafting` | 动态动作菜单（exercise/explore/...） | 由动态菜单驱动，统一动作面板模板 |
| `/game/warehouse` | `inventory` + 相关扩展面板 | 统一库存展示与操作跳转 |
| `/game/tasks` | `quest` | 对齐任务面板信息架构 |

## 10. 实施顺序（固定）

1. 落地菜单驱动层（含动态 resolver）
2. 接入 tab-query 状态同步与路由兼容层
3. 按 deep-article 菜单顺序逐面板迁移
4. 每个模块同步补齐六语言 i18n（`zh-CN/zh-TW/en-US/ja/ko/ru`）
5. 收口全局层（主题、断线、聊天、移动菜单）
6. 更新标准文档与验收矩阵

## 11. 验收清单（AI 执行必须自检）

1. 菜单顺序、分组、动态项位置是否与本规范一致。
2. 任意菜单是否可通过 `tab` 直达并在刷新后保持。
3. 后端未就绪功能是否“完整可见但不可提交”。
4. 新增可见文案是否均有 `zh-CN/zh-TW/en-US/ja/ko/ru` key。
5. 移动端是否可通过抽屉访问所有菜单分组。
6. 跨模块跳转（仓库 -> 市场/强化/分解）是否可达目标面板。
