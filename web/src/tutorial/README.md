# Tutorial System

Interactive step-by-step guide system built on **Driver.js v1.4.0**. Zero-coupling architecture: business components are never modified.

## Table of Contents

- [Architecture](#architecture)
- [Quick Start: Add a New Tutorial](#quick-start-add-a-new-tutorial)
- [Step Configuration](#step-configuration)
- [Target Selector Strategy](#target-selector-strategy)
- [Prerequisites](#prerequisites)
- [Mobile Handling](#mobile-handling)
- [i18n](#i18n)
- [Common Patterns](#common-patterns)
- [Debugging](#debugging)
- [File Structure](#file-structure)

---

## Architecture

```
Business Components (NEVER modified)
  │
  │  DOM (unchanged)
  ▼
src/tutorial/
  ├── configs/*.ts          ← Step definitions (CSS selectors + route + setup)
  ├── driver-factory.ts     ← Driver.js wrapper with async target resolution
  ├── store.ts              ← Pinia store (start/complete/reset/persist)
  ├── persistence.ts        ← localStorage per-user
  └── components/
      └── TutorialGuidePanel.vue   ← Guide page (list + start/replay)

Driver.js (external, MIT license)
  └── Handles overlay, spotlight, popover, keyboard, mobile
```

**Coupling direction**: Tutorial → Business DOM (one-way). Business components never import or know about the tutorial system.

### Key Design Decisions

1. **CSS selector targeting** — No `v-tutorial` directives, no `data-` attributes. Tutorial configs use CSS selectors to find elements. Business components stay 100% clean.

2. **Async target resolution** — Driver.js requires `element` to be sync, but tutorials need to navigate routes, open dialogs, and wait for DOM. Solution: `prepareTarget()` does async work BEFORE each step, then Driver.js reads the element synchronously.

3. **Prerequisite chains** — Tutorials unlock sequentially. E.g., must complete `character` before `actions`, must complete `actions` before `home`.

4. **Lazy loading** — Each tutorial config is a separate chunk, loaded on demand. Driver.js is in a shared vendor chunk.

---

## Quick Start: Add a New Tutorial

### 1. Create config file

```typescript
// src/tutorial/configs/my-feature-tutorial.ts
import type { TutorialConfig } from "../types"

export const myFeatureTutorial: TutorialConfig = {
  id: "my-feature",
  nameKey: "tutorial.myFeature.name",
  descriptionKey: "tutorial.myFeature.description",
  icon: "game-icons:star",  // Iconify icon ID
  version: 1,
  category: "core",  // "core" | "economy" | "progression" | "social"
  order: 10,  // Display order within category
  prerequisites: ["character"],  // Must complete these first
  steps: [
    {
      id: "my-feature-intro",
      target: {
        route: "/game/my-feature",  // Navigate here first
        selector: ".page-shell .font-bold",  // Find this element
        waitForMs: 5000,  // Max wait time (default: 5000)
      },
      popover: {
        titleKey: "tutorial.myFeature.intro.title",
        descriptionKey: "tutorial.myFeature.intro.desc",
        side: "bottom",  // "top" | "right" | "bottom" | "left" | "over"
      },
    },
    // ... more steps
  ],
}
```

### 2. Register in lazy loader

```typescript
// src/tutorial/configs/index.ts
const loaders: Record<string, TutorialLoader> = {
  // ... existing
  "my-feature": () => import("./my-feature-tutorial").then((m) => m.myFeatureTutorial),
}
```

### 3. Add i18n keys

```typescript
// src/locales/tutorial/en-US.ts
tutorial: {
  myFeature: {
    name: "My Feature",
    description: "Learn how to use my feature",
    intro: {
      title: "Welcome to My Feature",
      desc: "This is where you do amazing things.",
    },
  },
}
```

```typescript
// src/locales/tutorial/zh-CN.ts
tutorial: {
  myFeature: {
    name: "我的功能",
    description: "学习如何使用我的功能",
    intro: {
      title: "欢迎使用我的功能",
      desc: "在这里你可以做很酷的事情。",
    },
  },
}
```

### 4. Test

```typescript
// Browser console (after login)
const { useTutorialStore } = await import("@/tutorial")
const store = useTutorialStore()
await store.start("my-feature")
```

---

## Extend an Existing Tutorial

Adding more steps to a tutorial is intentionally cheap — it's data-only, no component
changes. To expand e.g. the actions tutorial:

1. **Add the step(s)** to the `steps` array in `configs/<id>-tutorial.ts`. Order in the
   array is the order shown. A step is just a target + popover:

   ```typescript
   {
     id: "action-new-thing",
     target: { selector: "[data-tutorial='action-new-thing']", waitForMs: 4000 },
     popover: { titleKey: "tutorial.actions.newThing.title", descriptionKey: "tutorial.actions.newThing.desc", side: "bottom" },
   }
   ```

2. **Add the i18n keys** (`tutorial.actions.newThing.title` / `.desc`) to **all six**
   locale files: `en-US, zh-CN, zh-TW, ja, ko, ru`. Missing keys render the raw key string.

3. **Bump the config `version`** (e.g. `version: 3 → 4`). Completion is tracked per
   `(id, version)`, so bumping re-flags the tutorial as "Replay" for users who already
   finished the old version — they'll see the new steps. Leave `version` unchanged only
   if the edit is cosmetic (typo/wording) and you don't want to re-prompt anyone.

4. **Add the DOM hook** if the step targets new UI: put a `data-tutorial="action-new-thing"`
   attribute on the business component (one-way coupling — the component never imports the
   tutorial system). Prefer a stable `data-tutorial` hook over CSS-class selectors.

That's it — no loader/registration changes (those are per-tutorial, not per-step).

**Things the framework already handles for you when extending:**
- **Responsive twins** — if the new target has a mobile + desktop variant sharing one
  `data-tutorial`, the resolver auto-picks the visible one (see *Responsive Duplicate
  Elements* below). No extra work.
- **Async / late-rendered targets** — set `waitForMs`; the resolver waits via
  `MutationObserver` until the element appears.
- **Setup before a step** — use `target.setup` (async) to open a dialog, switch a tab, or
  navigate first (see *Common Patterns*).
- **Conditional steps** — use `when: () => …` to include a step only when it applies
  (e.g. `hasOwnedKitties()`), and `optional: true` to not warn if the target is absent.

---

## Step Configuration

### TargetDescriptor

```typescript
type TargetDescriptor = {
  route?: string           // Navigate to this route before finding element
  selector?: string        // CSS selector to find the element
  setup?: () => Promise<void> | void  // Run after navigation (open dialog, switch tab, etc.)
  waitForMs?: number       // Max ms to wait for element (default: 5000)
}
```

### TutorialStep

```typescript
type TutorialStep = {
  id: string               // Unique step ID
  target: TargetDescriptor
  popover: {
    titleKey: string       // i18n key
    descriptionKey: string // i18n key
    side?: "top" | "right" | "bottom" | "left" | "over"
    align?: "start" | "center" | "end"
    popoverClass?: string  // Extra CSS class
  }
  when?: () => boolean     // Skip step if returns false
  mobile?: {               // Mobile-specific overrides
    target?: Partial<TargetDescriptor>
    popover?: Partial<TutorialPopover>
  }
}
```

### TutorialConfig

```typescript
type TutorialConfig = {
  id: string
  nameKey: string
  descriptionKey: string
  icon?: string            // Iconify icon ID
  steps: TutorialStep[]
  version?: number         // Increment to force re-completion
  prerequisites?: string[] // Tutorial IDs that must be completed first
  category?: "core" | "economy" | "progression" | "social"
  order?: number           // Display order within category
}
```

---

## Target Selector Strategy

### Choosing Selectors

**Good selectors** (stable, semantic):
```typescript
// Element Plus component structure (stable across versions)
".el-tabs__item:first-child"           // First tab
".el-tabs__item:nth-child(2)"          // Second tab
".el-button--primary"                  // Primary button

// Scoped to page (more specific)
".page-shell .font-bold"               // Page title
".page-shell .el-tabs__item"           // Tabs in this page
".page-shell .el-card:first-child"     // First card

// Component-specific (if available)
".action-panel-tabs .grid"             // Action cards grid
".equipment-main-grid"                 // Equipment slots
".skill-card__drag-handle"             // Skill drag handle
```

**Bad selectors** (fragile, implementation-specific):
```typescript
"div > div > div > span"               // Too deep, breaks on any change
".v-123abc"                             // Vue-generated class, changes on rebuild
"#app > div > main > section > div"    // Too deep
```

### Handling Dynamic Content

**Elements inside tabs** — The tab content may not be rendered until the tab is active:
```typescript
{
  target: {
    selector: ".el-tab-pane .my-content",
    setup: async () => {
      // Click the tab first
      const tab = document.querySelector<HTMLElement>(".el-tabs__item:nth-child(2)")
      tab?.click()
      await new Promise((r) => setTimeout(r, 400))  // Wait for tab switch
    },
    waitForMs: 6000,
  },
}
```

**Elements inside dialogs** — Dialogs are rendered lazily:
```typescript
{
  target: {
    selector: ".el-dialog .my-dialog-content",
    setup: async () => {
      // Click the button that opens the dialog
      const btn = document.querySelector<HTMLElement>(".open-dialog-btn")
      btn?.click()
      await new Promise((r) => setTimeout(r, 500))  // Wait for dialog to render
    },
  },
}
```

**Dynamic panel tabs** (e.g., `/game/panel/collecting`) — Must wait for shell's dynamic menu:
```typescript
{
  target: {
    selector: ".page-shell .action-panel-tabs",
    setup: async () => {
      // Use helper that waits for shell to load dynamic menu
      const { navigateToDynamicPanelTab } = await import("../driver-factory")
      await navigateToDynamicPanelTab("collecting")
    },
    waitForMs: 10000,
  },
}
```

### Conditional Steps

Skip steps based on game state:
```typescript
{
  id: "kitty-lane",
  target: { selector: ".kitty-lane" },
  when: () => document.querySelectorAll(".kitty-lane").length > 0,
  // Only show if kitty lanes exist
}
```

Mobile-only steps:
```typescript
{
  id: "mobile-role-bar",
  target: { selector: ".attributes-mobile-rolebar" },
  when: () => window.innerWidth <= 767,
}
```

---

## Prerequisites

Tutorials unlock sequentially. Define dependencies in `prerequisites`:

```typescript
{
  id: "market",
  prerequisites: ["warehouse"],  // Must complete warehouse first
}
```

The guide page shows locked tutorials with a 🔒 icon and prerequisite names. Users can't start locked tutorials.

### Common Prerequisite Chains

```
character
├── character-attributes
├── character-equipment
├── character-skills
└── actions → ┬── home
              ├── training → combat
              ├── tasks
              └── exploration → building
```

---

## Mobile Handling

Driver.js handles mobile automatically (popover repositioning, touch events). For custom mobile behavior:

### Mobile Overrides

```typescript
{
  id: "create-listing",
  target: { selector: ".create-btn" },
  popover: { side: "left" },  // Desktop
  mobile: {
    popover: { side: "top" },  // Mobile override
  },
}
```

### Mobile-Specific Steps

```typescript
{
  id: "mobile-menu",
  target: { selector: ".mobile-menu-btn" },
  when: () => window.innerWidth <= 767,  // Only on mobile
}
```

### Responsive Duplicate Elements (mobile + desktop share one `data-tutorial`)

Many pages render **two** variants of the same element — one for mobile, one for
desktop — and toggle them with CSS (`lg:hidden` / `hidden lg:block`). Both stay in
the DOM; only one is `display:none` at a given breakpoint. Example
(`views/game/attributes/index.vue`):

```html
<!-- appears FIRST in document order, visible only on mobile -->
<div class="… lg:hidden" data-tutorial="role-selector">…</div>
<!-- visible only on desktop -->
<div class="hidden … lg:block" data-tutorial="role-selector">…</div>
```

The tutorial resolver (`queryVisible` in `driver-factory.ts`) **automatically picks the
visible variant** — it skips matches with no client rects (`display:none`) and only
falls back to the first match if none are visible. So sharing one `data-tutorial`
hook across breakpoints is fine; the spotlight lands on whichever variant the current
device actually shows. No `mobile` override or `when` guard is needed for this case.

> If you ever need device-specific *targeting* (different element entirely, not just a
> hidden twin), use a `mobile.target.selector` override instead of duplicating hooks.

### CSS Customization

Mobile popover styles in `src/style.css`:
```css
@media (max-width: 767px) {
  .tutorial-popover-mobile.driver-popover {
    max-width: 90vw;
    font-size: 14px;
  }
  .tutorial-popover-mobile.driver-popover button {
    min-height: 44px;  // Apple HIG touch target
  }
}
```

---

## i18n

All user-facing strings use i18n keys. Add to both `en-US.ts` and `zh-CN.ts`:

```typescript
tutorial: {
  myFeature: {
    name: "My Feature",
    description: "Learn how to use my feature",
    step1: {
      title: "Step 1 Title",
      desc: "Step 1 description with <strong>HTML</strong> support.",
    },
  },
}
```

Other locales (`zh-TW`, `ja`, `ko`, `ru`) fall back to `en-US` if keys are missing.

---

## Common Patterns

### Pattern 1: Navigate to Route + Highlight Element

```typescript
{
  target: {
    route: "/game/market",
    selector: ".page-shell .el-tabs__item:first-child",
    waitForMs: 5000,
  },
}
```

### Pattern 2: Switch Tab + Highlight Content

```typescript
{
  target: {
    selector: ".tab-content",
    setup: async () => {
      const tab = document.querySelector<HTMLElement>(".el-tabs__item:nth-child(2)")
      tab?.click()
      await new Promise((r) => setTimeout(r, 400))
    },
  },
}
```

### Pattern 3: Open Dialog + Highlight Inside

```typescript
{
  target: {
    selector: ".el-dialog .dialog-content",
    setup: async () => {
      const btn = document.querySelector<HTMLElement>(".open-dialog-btn")
      btn?.click()
      await new Promise((r) => setTimeout(r, 500))
    },
    waitForMs: 6000,
  },
}
```

### Pattern 4: Navigate to Dynamic Panel Tab

```typescript
{
  target: {
    selector: ".action-panel-tabs",
    setup: async () => {
      const { navigateToDynamicPanelTab } = await import("../driver-factory")
      await navigateToDynamicPanelTab("collecting")
    },
    waitForMs: 10000,
  },
}
```

### Pattern 5: Click Element + Wait for Next Page

```typescript
{
  target: {
    selector: ".next-page-content",
    setup: async () => {
      const btn = document.querySelector<HTMLElement>(".next-page-btn")
      btn?.click()
      await new Promise((r) => setTimeout(r, 600))
    },
    waitForMs: 8000,
  },
}
```

---

## Debugging

### Test a Tutorial in Console

```typescript
// Start tutorial
const { useTutorialStore } = await import("@/tutorial")
const store = useTutorialStore()
await store.start("market")

// Check completion status
console.log(store.isCompleted("market", 1))

// Reset tutorial (allows replay)
store.reset("market")
```

### Check Prerequisites

```typescript
const { loadTutorialConfig } = await import("@/tutorial")
const config = await loadTutorialConfig("market")
console.log("Prerequisites:", config?.prerequisites)
console.log("Unlocked:", store.isUnlocked(config!))
```

### Inspect Target Resolution

```typescript
// Manually test target resolution
const { navigateToDynamicPanelTab } = await import("@/tutorial/driver-factory")
await navigateToDynamicPanelTab("collecting")
// Check if element exists
console.log(document.querySelector(".action-panel-tabs"))
```

### Common Issues

**Element not found**:
- Increase `waitForMs` (default 5000ms may not be enough for slow loads)
- Check if element is inside a tab/dialog that needs to be opened first (use `setup`)
- Verify selector matches actual DOM (use browser DevTools)

**Route redirects to homepage**:
- For dynamic panel tabs (`/game/panel/*`), use `navigateToDynamicPanelTab()` helper
- Shell's `allowedTabSet` may not include the tab yet (async loading)

**Popover positioned wrong**:
- Change `side` property (try different sides)
- Use `mobile` overrides for different mobile positioning

**Step skipped unexpectedly**:
- Check `when()` condition — it may return false
- Check if prerequisite tutorials are completed

---

## File Structure

```
src/tutorial/
├── README.md                        ← This file
├── types.ts                         ← TypeScript interfaces
├── store.ts                         ← Pinia store
├── driver-factory.ts                ← Driver.js wrapper + navigateToDynamicPanelTab
├── persistence.ts                   ← localStorage persistence
├── index.ts                         ← Public API exports
├── components/
│   └── TutorialGuidePanel.vue       ← Guide page
└── configs/
    ├── index.ts                     ← Lazy registry
    ├── character-tutorial.ts
    ├── character-attributes-tutorial.ts
    ├── character-equipment-tutorial.ts
    ├── character-skills-tutorial.ts
    ├── character-battle-items-tutorial.ts
    ├── character-loadouts-tutorial.ts
    ├── actions-tutorial.ts
    ├── home-tutorial.ts
    ├── shop-tutorial.ts
    ├── warehouse-tutorial.ts
    ├── market-tutorial.ts
    ├── auction-tutorial.ts
    ├── enhance-tutorial.ts
    ├── affix-tutorial.ts
    ├── combat-tutorial.ts
    ├── tasks-tutorial.ts
    ├── building-tutorial.ts
    ├── crafting-tutorial.ts
    ├── cattery-tutorial.ts
    └── settings-tutorial.ts
```

### Key Files

- **`types.ts`** — All TypeScript interfaces (`TutorialConfig`, `TutorialStep`, `TargetDescriptor`, etc.)
- **`store.ts`** — Pinia store with `start()`, `isCompleted()`, `isUnlocked()`, `reset()`
- **`driver-factory.ts`** — Creates Driver.js instance from `TutorialConfig`. Exports `navigateToDynamicPanelTab()` helper.
- **`configs/index.ts`** — Lazy registry. Each tutorial is loaded on demand via dynamic import.
- **`components/TutorialGuidePanel.vue`** — Guide page that lists all tutorials with locked/completed state.

### Modified Files (outside `src/tutorial/`)

- **`src/views/game/shell/index.vue`** — Uncommented `gameguide` menu item (line 530)
- **`src/views/game/panel/index.vue`** — Added `gameguide` tab handling + `TutorialGuidePanel`
- **`src/locales/tutorial/en-US.ts`** + **`src/locales/tutorial/zh-CN.ts`** — Tutorial i18n keys
- **`src/style.css`** — Driver.js mobile CSS overrides
- **`package.json`** — Added `driver.js@1.4.0` dependency

---

## API Reference

### `useTutorialStore()`

```typescript
const store = useTutorialStore()

// Start a tutorial
await store.start("market")

// Check if completed
const done = store.isCompleted("market", 1)  // (id, version)

// Check if unlocked (prerequisites met)
const unlocked = store.isUnlocked(config)

// Reset tutorial (allows replay)
store.reset("market")

// Load all configs (for guide page)
const all = await store.loadAll()
```

### `navigateToDynamicPanelTab(tabId)`

Waits for shell's dynamic menu to load, then navigates to a panel tab. Use for dynamic action tabs like `collecting`, `crafting`, etc.

```typescript
const { navigateToDynamicPanelTab } = await import("@/tutorial/driver-factory")
await navigateToDynamicPanelTab("collecting")
```

### `loadTutorialConfig(id)`

Loads a tutorial config by ID (lazy).

```typescript
const { loadTutorialConfig } = await import("@/tutorial")
const config = await loadTutorialConfig("market")
```

### `getAllTutorialIds()`

Returns all registered tutorial IDs.

```typescript
const { getAllTutorialIds } = await import("@/tutorial")
const ids = getAllTutorialIds()  // ["character", "actions", "market", ...]
```

---

## Tips

1. **Start simple** — Add one step, test it, then add more. Don't try to write the entire tutorial at once.

2. **Use browser DevTools** — Inspect elements to find stable selectors. Check the DOM structure before writing selectors.

3. **Test on mobile** — Resize browser to <767px and test. Mobile layouts may need different selectors or `mobile` overrides.

4. **Increment `version`** — If you update a tutorial's steps, increment `version` so users who completed the old version see it again.

5. **Keep selectors scoped** — Use `.page-shell` prefix to scope selectors to the current page. Avoid overly specific selectors.

6. **Use `setup` for side effects** — If you need to click a button, switch a tab, or open a dialog, do it in `setup`, not in the target.

7. **Wait for async operations** — If an action triggers an API call or animation, add a `delay` in `setup` to wait for it to complete.

8. **Test prerequisites** — Use `store.reset()` to clear completion status and test the full prerequisite chain.

---

## License

Driver.js is MIT licensed. Tutorial system is part of the game codebase.
