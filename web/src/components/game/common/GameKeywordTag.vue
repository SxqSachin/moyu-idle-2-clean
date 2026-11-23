<template>
  <el-popover
    :visible="visible"
    :width="isMobile ? 286 : 320"
    placement="auto-start"
    :fallback-placements="['left-start', 'right-start', 'top-start', 'bottom-start']"
    :boundaries-padding="10"
    effect="light"
    teleported
    popper-class="game-keyword-popper"
  >
    <template #reference>
      <el-tag
        class="game-keyword-tag"
        effect="light"
        size="small"
        tabindex="0"
        role="button"
        :aria-label="t('game.keyword.open', { keyword: keyword.name })"
        :aria-expanded="visible"
        @mouseenter="hovered = true"
        @mouseleave="hovered = false"
        @focus="focused = true"
        @blur="close"
        @click.stop="pinned = !pinned"
        @keydown.enter.stop.prevent="pinned = !pinned"
        @keydown.space.stop.prevent="pinned = !pinned"
        @keydown.esc.stop.prevent="close"
      >
        {{ label || keyword.name }}
      </el-tag>
    </template>
    <div class="game-keyword-card" role="tooltip">
      <strong>{{ keyword.name }}</strong>
      <p>{{ keyword.description }}</p>
      <dl v-if="keyword.trigger || keyword.source">
        <template v-if="keyword.trigger">
          <dt>{{ t("game.keyword.details") }}</dt>
          <dd>{{ keyword.trigger }}</dd>
        </template>
        <template v-if="keyword.source">
          <dt>{{ t("game.keyword.source") }}</dt>
          <dd>{{ keyword.source }}</dd>
        </template>
      </dl>
    </div>
  </el-popover>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { useI18n } from "vue-i18n"
import { useDevice } from "@composables/game/useDevice"
import type { GameKeywordDisplay } from "@services/game-keyword"

defineProps<{
  keyword: GameKeywordDisplay
  label?: string
}>()

const { t } = useI18n()
const { isMobile } = useDevice()
const hovered = ref(false)
const focused = ref(false)
const pinned = ref(false)
const visible = computed(() => hovered.value || focused.value || pinned.value)

function close() {
  focused.value = false
  pinned.value = false
}
</script>

<style scoped>
.game-keyword-tag {
  height: 20px;
  margin: 0 2px;
  border-color: color-mix(in srgb, var(--app-primary) 44%, var(--app-border));
  background: color-mix(in srgb, var(--app-primary) 9%, var(--app-surface));
  padding-inline: 5px;
  color: var(--app-primary);
  font-size: 11px;
  line-height: 18px;
  vertical-align: baseline;
  cursor: help;
}

.game-keyword-tag:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--app-primary) 55%, transparent);
  outline-offset: 2px;
}

.game-keyword-card {
  display: grid;
  gap: 7px;
  color: var(--app-text);
}

.game-keyword-card strong {
  font-size: 14px;
  letter-spacing: 0;
}

.game-keyword-card p,
.game-keyword-card dl,
.game-keyword-card dd {
  margin: 0;
}

.game-keyword-card p,
.game-keyword-card dd {
  color: var(--app-text-secondary);
  font-size: 12px;
  line-height: 1.55;
}

.game-keyword-card dl {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 5px 8px;
  border-top: 1px solid var(--app-border);
  padding-top: 7px;
}

.game-keyword-card dt {
  color: var(--app-text-muted);
  font-size: 11px;
}
</style>
