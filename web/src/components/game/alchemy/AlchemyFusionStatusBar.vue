<template>
  <aside class="inherit-status-bar" data-tutorial="alchemy-inherit-summary">
    <div
      v-for="card in cards"
      :key="card.key"
      class="inherit-status-card"
      :class="{ 'is-over': card.over, 'has-risk': card.risk }"
    >
      <div class="inherit-status-card__head">
        <span>{{ card.title }}</span>
        <el-popover v-if="card.tooltip" placement="bottom" :width="280" trigger="hover" :show-after="180">
          <template #reference>
            <el-icon class="inherit-help-icon"><QuestionFilled /></el-icon>
          </template>
          <div class="inherit-help-popover">
            <strong>{{ card.title }}</strong>
            <span>{{ card.tooltip }}</span>
          </div>
        </el-popover>
      </div>
      <strong>{{ card.value }}</strong>
      <small>{{ card.hint }}</small>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { QuestionFilled } from "@element-plus/icons-vue"

type StatusCard = {
  key: string
  title: string
  value: string
  hint: string
  tooltip?: string
  over?: boolean
  risk?: boolean
}

defineProps<{
  cards: StatusCard[]
}>()
</script>

<style scoped>
.inherit-status-bar {
  position: sticky;
  top: 8px;
  z-index: 40;
  align-self: start;
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  padding: 5px;
  border: 1px solid color-mix(in srgb, var(--app-border, #dde4ea) 82%, transparent);
  border-radius: 7px;
  background: color-mix(in srgb, var(--app-surface, #fff) 97%, var(--app-bg, #f6f8fb));
  box-shadow: 0 3px 12px rgba(15, 23, 42, 0.06);
}

.inherit-status-card {
  min-width: 0;
  flex: 1 1 150px;
  display: grid;
  gap: 0;
  padding: 5px 7px;
  border-left: 2px solid transparent;
  border-radius: 5px;
  background: transparent;
}

.inherit-status-card:hover {
  background: color-mix(in srgb, var(--app-bg, #f6f8fb) 64%, transparent);
}

.inherit-status-card__head {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 4px;
}

.inherit-status-card__head span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inherit-status-card span {
  color: var(--app-muted, #667085);
  font-size: 11px;
  font-weight: 800;
}

.inherit-help-icon {
  flex: 0 0 auto;
  color: color-mix(in srgb, var(--app-muted, #667085) 82%, transparent);
  font-size: 12px;
  cursor: help;
}

.inherit-help-icon:hover {
  color: var(--el-color-primary, #409eff);
}

.inherit-help-popover {
  display: grid;
  gap: 5px;
  color: var(--app-text, #1f2328);
  font-size: 12px;
  line-height: 1.45;
}

.inherit-help-popover span {
  color: var(--app-muted, #667085);
}

.inherit-status-card strong {
  min-width: 0;
  color: var(--app-text, #1f2328);
  font-size: 13px;
  line-height: 1.25;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inherit-status-card small {
  min-width: 0;
  color: var(--app-muted, #667085);
  font-size: 10px;
  line-height: 1.25;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inherit-status-card.is-over,
.inherit-status-card.has-risk {
  border-left-color: var(--el-color-danger, #f56c6c);
  background: color-mix(in srgb, var(--el-color-danger, #f56c6c) 8%, var(--app-surface, #fff));
}

.inherit-status-card.is-over strong,
.inherit-status-card.has-risk strong {
  color: var(--el-color-danger, #f56c6c);
}

@media (max-width: 1100px) {
  .inherit-status-bar {
    gap: 4px;
  }
}

@media (max-width: 700px) {
  .inherit-status-card {
    flex-basis: calc(50% - 4px);
  }
}
</style>
