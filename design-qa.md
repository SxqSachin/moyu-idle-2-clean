# Fishing Ecology Dialog Design QA

## Scope

- Selected source: `（生成工作文件）`
- Implementation: `web/src/views/game/fishing/index.vue`
- Interaction coverage: `web/e2e/fishing.spec.ts`
- Desktop capture: `output/playwright/fishing-desktop-ecology.png`
- Mobile capture: `output/playwright/fishing-mobile-ecology.png`
- Focused source/implementation comparison: `output/playwright/fishing-ecology-design-qa.png`

## Capture Conditions

- Source pixels: 1452 x 1083.
- Desktop implementation: 1440 x 900 CSS pixels, DPR 1, light theme.
- Mobile implementation: 390 x 844 CSS pixels, DPR 1, light theme.
- State: authenticated mocked player path, 06:30 water time, six forecast points, and two strange-fish windows.

## Comparison

- Preserved the selected direction: a wide information-led dialog, categorical tide line, restrained lake-teal accent, and warm-gold strange-fish windows.
- Kept the app's real labels and forecast data instead of baking reference content into the UI.
- Did not copy the reference's `current` badge onto 07:00: the runtime state is 06:30, so marking the first future forecast as current would be false.
- The implementation is intentionally denser than the concept image to remain consistent with the game's operational UI.

## Iterations

1. Initial implementation was 820 px wide, sat too low, mixed success green with teal, and used outline tide nodes.
2. Final implementation is 1040 px wide at 1440 px, vertically centered, uses one teal state color, filled nodes, larger fish art, and a stronger close control.
3. Long localized labels now wrap inside their own grid cells. The forecast region is keyboard-focusable and gains a visible focus ring.
4. At 768-879 px the forecast becomes a keyboard-scrollable compact strip. At 390 px the dialog body scrolls vertically, the strip scrolls horizontally, and the document does not overflow.

## Verification

- Desktop panel width and vertical centering are asserted from its rendered bounding box.
- Desktop forecast has no horizontal overflow at 1440 px.
- Compact and mobile forecasts expose all six points through horizontal scrolling.
- Mobile body scrolling reaches the second strange-fish row while background page scroll remains locked.
- Dialog accessible name, four condition cells, six forecast points, two strange-fish rows, and teleported theme tokens are asserted.
- The local dev route produced no console warnings or errors in the in-app browser. Modal-specific behavior was verified on the mocked Playwright player path because the in-app route stopped at authentication.

## Final Result

passed
