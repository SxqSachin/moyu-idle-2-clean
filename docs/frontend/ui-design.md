# UI Design（Web）

## TL;DR（先读这里）

- 视觉基调：Apple 风格的简洁克制；交互组件统一用 Element Plus；布局与样式统一用 Tailwind。
- 新页面必须先满足移动端，再做大屏增强。
- 本文偏视觉与样式基线；玩法信息架构优先看 `docs/frontend/gameplay-ui-standard.md`。

## 何时阅读本文

- 你在改：主题色、字体、间距、组件外观、暗色模式、响应式布局。
- 你在统一：跨页面视觉一致性与 UI 基线。

## 常见任务直达

- 色板与变量：看“Colors”
- 字体与排版：看“Typography”（若存在）
- 组件风格与状态：看对应组件章节

## Overview

本项目 UI 整体视觉风格**参考 Apple 的简洁、克制哲学**，但不刻意复刻 Apple 的具体实现。交互组件直接使用 **Element Plus**（按钮、表单、弹窗、表格、消息提示等），样式层通过 **Tailwind CSS** 工具类覆写和扩展。这样可以获得 Apple 风格的"轻盈感"，同时保有 Element Plus 完整的无障碍和交互能力。

> 玩法页面（首页、动作卡、仓库、强化链路）的信息架构与交互执行规范请优先阅读：`docs/frontend/gameplay-ui-standard.md`。本文件侧重视觉与样式基线。

**设计方向摘要：**
- 视觉以"留白 + 低噪音"为基调：颜色克制，阴影极少，排版清晰。
- Element Plus 组件作为交互基础单元，不重新实现；通过 `el-config-provider` 和 Tailwind 在样式层调整外观。
- 主题色沿用单一蓝色系 (`#0066cc`)，配合深灰近黑文字，保持 Apple 的品牌感知。
- 字体优先走系统字体栈（macOS/iOS 上自动用 SF Pro），非 Apple 平台降级到 Inter 或 system-ui。
- 响应式布局用 Tailwind 断点（`sm` / `md` / `lg` / `xl` / `2xl`）统一管理，不再定义额外断点命名。
- **移动端优先是强制规则**：所有新页面和改版必须先完成小屏信息架构与交互，再做大屏增强。
- 深色模式：通过 Tailwind `dark:` 变体 + Element Plus 暗色主题（`ep-dark` class）实现，双方各司其职。

**技术栈约定：**
- **Element Plus** — 提供所有有状态交互组件（`el-button`、`el-input`、`el-dialog`、`el-table`、`el-form`、`el-message`、`el-tooltip` 等）。
- **Tailwind CSS** — 提供所有布局、间距、颜色、排版工具类；通过 `@layer components` 定义少量复合样式。
- **CSS Variables** — Element Plus 的主题色通过 `:root` 覆写 `--el-color-primary` 等变量；Tailwind 配置中同步引用同一组 CSS 变量以保证一致性。

## Colors

> 颜色系统通过 CSS 变量统一管理，Tailwind `tailwind.config.js` 中 `theme.extend.colors` 直接引用这些变量，Element Plus 主题也通过覆写 `--el-color-primary` 系列变量接入同一色板。

### 主题色配置示例（`src/style.css`）

```css
:root {
  --color-primary:        #0066cc;
  --color-primary-light:  #2997ff;   /* 深色背景上使用 */
  --color-primary-dark:   #0051a3;   /* hover/active 加深 */
  --color-ink:            #1d1d1f;
  --color-ink-muted:      #6e6e73;
  --color-canvas:         #ffffff;
  --color-canvas-2:       #f5f5f7;
  --color-surface-dark:   #1c1c1e;
  --color-border:         #d2d2d7;
  --color-border-light:   #e8e8ed;

  /* Element Plus 主题对接 */
  --el-color-primary:     var(--color-primary);
}
```

```js
// tailwind.config.js
theme: {
  extend: {
    colors: {
      primary:        'var(--color-primary)',
      'primary-light':'var(--color-primary-light)',
      'primary-dark': 'var(--color-primary-dark)',
      ink:            'var(--color-ink)',
      'ink-muted':    'var(--color-ink-muted)',
      canvas:         'var(--color-canvas)',
      'canvas-2':     'var(--color-canvas-2)',
      'surface-dark': 'var(--color-surface-dark)',
      border:         'var(--color-border)',
      'border-light': 'var(--color-border-light)',
    }
  }
}
```

### 色板说明

| Token | 值 | 用途 |
|---|---|---|
| `primary` | #0066cc | 唯一主交互色：按钮、链接、选中态、聚焦环 |
| `primary-light` | #2997ff | 深色背景上的链接/强调 |
| `primary-dark` | #0051a3 | 主按钮 hover/active |
| `ink` | #1d1d1f | 所有正文、标题 |
| `ink-muted` | #6e6e73 | 次要文字、placeholder、禁用 |
| `canvas` | #ffffff | 主背景 |
| `canvas-2` | #f5f5f7 | 次级背景（页脚、侧边栏、卡片底色区分） |
| `surface-dark` | #1c1c1e | 深色区块背景 |
| `border` | #d2d2d7 | 常规边框（Element Plus 输入框、卡片） |
| `border-light` | #e8e8ed | 浅分割线 |

**原则：不引入第二个品牌色。** 状态色（成功绿、警告橙、危险红）沿用 Element Plus 默认语义色，不纳入品牌色板。


**无装饰性渐变。** 页面氛围来自插图/图片资产，不使用 CSS 渐变作为背景装饰。

## Typography

### 字体栈

```css
font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Inter', sans-serif;
```

- macOS / iOS：自动解析为 SF Pro，保留 Apple 手感。
- 其他平台：降级至 Inter（可通过 Google Fonts variable font 引入）。
- Tailwind 配置中设置 `fontFamily.sans` 指向以上栈即可，无需额外处理。

### 字阶（对应 Tailwind 工具类）

| 用途 | 大小 | 字重 | 行高 | 字间距 | Tailwind 类参考 |
|---|---|---|---|---|---|
| 大标题 | 40–48px | 600 | 1.1 | -0.02em | `text-4xl font-semibold tracking-tight` |
| 页面标题 | 28–34px | 600 | 1.15 | -0.015em | `text-3xl font-semibold tracking-tight` |
| 区块标题 | 21–24px | 600 | 1.25 | -0.01em | `text-2xl font-semibold` |
| 副标题 | 17–18px | 600 | 1.35 | 0 | `text-lg font-semibold` |
| 正文 | 15–16px | 400 | 1.5 | 0 | `text-base` |
| 说明/标签 | 13–14px | 400 | 1.4 | 0 | `text-sm text-ink-muted` |
| 微型提示 | 12px | 400 | 1.3 | 0 | `text-xs text-ink-muted` |

**原则：**
- 大标题保持 `tracking-tight`（负字间距），正文不收紧。
- 字重仅用 400 / 600，避免 500（视觉层次不够清晰）。
- Element Plus 组件内部字号由其自身 CSS 变量控制，覆写时通过 `--el-font-size-*` 系列变量调整，不直接 hack 组件 DOM。

## Layout

### 间距

使用 Tailwind 默认 4px 基础栅格（`p-1` = 4px）。常用结构间距：

| 语义 | 值 | Tailwind |
|---|---|---|
| 卡片内边距 | 20–24px | `p-5` / `p-6` |
| 区块垂直间距 | 48–64px | `py-12` / `py-16` |
| 页面侧边距（移动） | 16px | `px-4` |
| 页面侧边距（桌面） | 24–32px | `px-6` / `px-8` |
| 列表项间距 | 8–12px | `gap-2` / `gap-3` |
| 元素组间距 | 16px | `gap-4` |

### 强制布局边界

- 首页是执行队列唯一展示页；其他页面禁止展示执行队列卡片。
- 关键路径交互（执行、确认、取消、拖放替代入口）必须在触控设备可完成，禁止仅 hover 可见的核心控件。
- 仓库列表与详情组件必须可通过统一“快捷组件”嵌入任意页面，支持高频查看与快捷操作。

### 组件架构（单一职责）

- 列表展示组件：只负责物品渲染与交互事件派发，不直接发起 API 请求。
- 页面容器组件：只负责状态管理、接口调用、冲突重试、错误提示与埋点。
- 类型与协议组件：列表/操作载荷类型单独维护，避免页面重复声明。
- 约束：同一套仓库交互不得在不同页面复制实现，必须复用统一组件。

### 容器

```html
<div class="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8">...</div>
```

游戏 UI 内容宽度上限约 960px，不需要 Apple 的 1440px 满铺。侧边栏布局使用 `grid grid-cols-[240px_1fr]` 或 Tailwind `lg:grid-cols-[260px_1fr]`。

### 网格

- 卡片列表：`grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4`
- 表单布局：`grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4`
- 全宽区块：不设 max-w，用背景色区分，保持整洁

## Elevation & Depth

| 层级 | 实现方式 | 用途 |
|---|---|---|
| 平面 | 无阴影无边框 | 页面背景、导航栏、区块容器 |
| 分割线 | `border border-border-light` | 卡片、输入框、表格行 |
| 轻浮起 | `shadow-sm`（Tailwind） | 信息卡、选项卡 hover |
| 模态/抽屉 | Element Plus 默认阴影 | `el-dialog`、`el-drawer` 保持默认，不覆写 |
| 毛玻璃导航 | `backdrop-blur-md bg-canvas/80` | 顶部导航栏 sticky 时 |

**不给按钮加阴影。** 按钮的层次感通过背景色和边框体现，不叠加 `box-shadow`。

## Shapes

### 圆角

| 语义 | 值 | Tailwind | 用途 |
|---|---|---|---|
| 无圆角 | 0 | `rounded-none` | 全宽背景块 |
| 小圆角 | 6px | `rounded-md` | 工具按钮、标签、徽标 |
| 中圆角 | 10px | `rounded-xl` | 卡片、对话框、输入框 |
| 大圆角 | 16px | `rounded-2xl` | 内容卡片、面板 |
| 胶囊 | 9999px | `rounded-full` | 主要 CTA 按钮、搜索框、状态标签 |

Element Plus 组件的圆角通过 `--el-border-radius-base`（默认 4px）覆写：

```css
:root {
  --el-border-radius-base: 8px;
  --el-border-radius-small: 6px;
  --el-border-radius-round: 9999px;
}
```

## Components

> **约定：** 所有有状态交互（点击、输入、选择、校验、弹窗）均使用 Element Plus 组件。外观用 Tailwind 调整，交互逻辑不重新实现。

### 顶部导航

```html
<nav class="sticky top-0 z-50 h-11 flex items-center justify-between
            px-6 bg-surface-dark/90 backdrop-blur-md border-b border-border-light/10">
  <!-- logo + 主导航链接 + 右侧工具区 -->
</nav>
```

- 背景：深色半透明 + `backdrop-blur`，视觉参考 Apple global-nav 但颜色可适当调浅（游戏类不需要纯黑）。
- 链接：`text-sm text-white/70 hover:text-white transition-colors`
- 右侧：`el-button` size="small" type="primary"（圆角继承 `--el-border-radius-round`）

### 按钮

使用 `el-button`，通过 CSS 变量控制主题。不自定义按钮组件。

| 场景 | 用法 | 样式补充 |
|---|---|---|
| 主要行动 | `<el-button type="primary" round>` | 继承主题色，不额外叠加 class |
| 次要行动 | `<el-button round>` | 默认 outline 风格 |
| 危险/删除 | `<el-button type="danger">` | 保持 Element Plus 默认 |
| 图标按钮 | `<el-button :icon="Icon" circle>` | 44×44 最小点击区域 |
| 工具栏按钮 | `<el-button size="small">` | 紧凑型，`rounded-md` |

### 表单 & 输入

完全使用 Element Plus 表单体系（`el-form`、`el-form-item`、`el-input`、`el-select`、`el-checkbox` 等）。

布局用 Tailwind Grid：

```html
<el-form :model="form" label-position="top" class="grid grid-cols-1 md:grid-cols-2 gap-x-6">
  <el-form-item label="用户名" prop="username">
    <el-input v-model="form.username" />
  </el-form-item>
</el-form>
```

- `label-position="top"` — 更简洁，符合 Apple "标签在上" 习惯。
- 错误态、校验消息完全由 Element Plus 处理，不另外实现。

### 卡片

Element Plus 没有强制性的卡片组件（`el-card` 可用但样式偏重），推荐用 Tailwind 直接写：

```html
<div class="bg-canvas rounded-2xl border border-border p-5 hover:shadow-sm transition-shadow">
  <!-- 内容 -->
</div>
```

深色区块：`bg-surface-dark text-white rounded-2xl p-5`

### 弹窗 & 抽屉

直接使用 `el-dialog` / `el-drawer`，覆写圆角和内边距：

```css
.el-dialog {
  border-radius: 16px;
  padding: 24px;
}
```

不额外包装，不重新实现关闭逻辑。

### 消息 & 通知

`el-message`、`el-notification` 保持默认行为，仅通过 `--el-color-primary` 保证主题色一致。

### 表格

`el-table` 保持默认，通过以下 Tailwind 覆写让边框更轻：

```css
.el-table {
  --el-table-border-color: var(--color-border-light);
  --el-table-header-bg-color: var(--color-canvas-2);
}
```

### 选项卡 / 标签页

使用 `el-tabs`，配置 `type="border-card"` 或默认 `type=""` 根据场景选择。视觉上无需大改，主题色对接即可。

## Do's and Don'ts

### Do
- 使用 `el-button`、`el-input` 等 Element Plus 组件处理所有交互，不重造轮子。
- 用 Tailwind 工具类控制布局、间距、颜色，CSS 写在 `@layer components` 里。
- 所有颜色引用 CSS 变量（`var(--color-primary)`），Tailwind 配置中映射，不硬编码 hex。
- 保持单一主题蓝 `#0066cc` 作为唯一主交互色，状态色（成功/警告/危险）使用 Element Plus 语义色。
- 大标题使用 `tracking-tight`（负字间距）营造紧凑感；正文使用 `leading-relaxed`。
- 深浅区块交替营造节奏感，颜色变化本身即是分隔符，不需要额外 divider。

### Don't
- 不引入第二个品牌色，不用彩色背景装饰。
- 不用 `box-shadow` 装饰按钮和卡片，阴影只出现在浮层（modal、dropdown）。
- 不重写 Element Plus 组件的交互逻辑，只覆写样式变量。
- 不使用 CSS 渐变作为区块背景，渐变只出现在图片/插图资产中。
- 不用 `font-medium`（weight 500），字重只走 400 / 600。
- 不在页面主体内容区域使用 `rounded-none`（保留给全宽背景块），内容卡片至少 `rounded-xl`。

## Responsive Behavior

使用 Tailwind 内建断点，不额外定义：

| Tailwind 断点 | 宽度 | 主要变化 |
|---|---|---|
| (默认 mobile) | < 640px | 单列布局；导航折叠为抽屉；标题缩至 `text-2xl` |
| `sm` | ≥ 640px | 2 列卡片网格；侧边距增至 `px-6` |
| `md` | ≥ 768px | 表单 2 列；出现侧边栏 |
| `lg` | ≥ 1024px | 3–4 列卡片；侧边栏固定展开 |
| `xl` | ≥ 1280px | 内容区锁定宽度，边距自动吸收 |

**Element Plus 响应式：** 使用 `el-row` / `el-col` 的 `:xs`、`:sm`、`:md`、`:lg` 属性处理栅格响应，或直接用 Tailwind Grid（两者可混用）。

移动端触控目标最小 44×44px（`el-button` 默认高度 32–40px，移动端场景用 `size="large"` 或加 `min-h-[44px]`）。

## Implementation Notes

### Element Plus 主题接入

```css
/* src/style.css */
:root {
  --el-color-primary:        #0066cc;
  --el-color-primary-light-3: #3388e0;
  --el-color-primary-dark-2:  #0051a3;
  --el-border-radius-base:   8px;
  --el-border-radius-round:  9999px;
  --el-font-size-base:       15px;
}
```

### Tailwind 配置

```js
// tailwind.config.js
theme: {
  extend: {
    colors: {
      primary: 'var(--color-primary)',
      ink: 'var(--color-ink)',
      'ink-muted': 'var(--color-ink-muted)',
      canvas: 'var(--color-canvas)',
      'canvas-2': 'var(--color-canvas-2)',
      'surface-dark': 'var(--color-surface-dark)',
      border: 'var(--color-border)',
      'border-light': 'var(--color-border-light)',
    },
    fontFamily: {
      sans: ['system-ui', '-apple-system', 'BlinkMacSystemFont', 'Inter', 'sans-serif'],
    },
  }
}
```

### 深色模式

在 `html` 元素上同时挂 Tailwind 的 `dark` class 和 Element Plus 的 `dark` class（两者约定名称相同），通过 `el-config-provider` 的 `:namespace` 或直接 class 切换。

```ts
// 切换深色模式
document.documentElement.classList.toggle('dark')
```

CSS 变量在 `.dark` 作用域内重新赋值即可完成深色适配。
