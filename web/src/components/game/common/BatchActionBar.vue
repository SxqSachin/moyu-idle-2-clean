<template>
  <Teleport to="body">
    <Transition name="batch-bar">
      <div v-if="visible" class="batch-action-bar" :data-tutorial="tutorialId || undefined">
        <span class="batch-action-bar__hint">
          <slot name="hint" />
        </span>
        <div class="batch-action-bar__buttons">
          <slot />
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    visible: boolean
    tutorialId?: string
  }>(),
  {
    tutorialId: ""
  }
)
</script>

<style>
.batch-action-bar {
  position: fixed;
  bottom: 54px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 2000;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 12px;
  max-width: calc(100vw - 24px);
  padding: 10px 16px;
  background: var(--el-bg-color-overlay);
  border: 1px solid var(--el-border-color-light);
  border-radius: 24px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
}

.batch-action-bar__hint {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
}

.batch-action-bar__buttons {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
}

@media (max-width: 640px) {
  .batch-action-bar {
    bottom: 48px;
    gap: 8px;
    padding: 12px 16px;
    padding-bottom: calc(12px + env(safe-area-inset-bottom));
  }

  .batch-action-bar__buttons .el-button {
    margin-left: 0;
  }
}

.batch-bar-enter-active,
.batch-bar-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.batch-bar-enter-from,
.batch-bar-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(12px);
}

@media (max-width: 640px) {
  .batch-bar-enter-from,
  .batch-bar-leave-to {
    transform: translateY(100%);
  }
}
</style>
