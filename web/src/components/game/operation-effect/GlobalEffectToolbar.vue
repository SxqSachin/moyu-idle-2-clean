<template>
  <div v-if="activeEffects.length" class="global-effect-toolbar">
    <el-popover
      v-if="!isMobile"
      v-model:visible="popoverVisible"
      placement="bottom-end"
      trigger="click"
      :width="380"
      popper-class="global-effect-toolbar-popper"
    >
      <template #reference>
        <button
          type="button"
          class="global-effect-toolbar__trigger"
          :aria-label="t('game.operationEffects.toolbarLabel', { count: activeEffects.length })"
          :title="t('game.operationEffects.toolbarLabel', { count: activeEffects.length })"
          :aria-expanded="popoverVisible"
        >
          <Icon icon="lucide:sparkles" />
          <span class="global-effect-toolbar__count" aria-hidden="true">{{ effectCountLabel }}</span>
        </button>
      </template>
      <div class="global-effect-toolbar__panel">
        <div class="global-effect-toolbar__heading">
          <span>{{ t("game.operationEffects.title") }}</span>
          <strong>{{ activeEffects.length }}</strong>
        </div>
        <OperationEffectList :items="activeEffects" :now-ms="nowMs" />
      </div>
    </el-popover>

    <button
      v-else
      type="button"
      class="global-effect-toolbar__trigger"
      :aria-label="t('game.operationEffects.toolbarLabel', { count: activeEffects.length })"
      :aria-expanded="drawerVisible"
      @click="drawerVisible = true"
    >
      <Icon icon="lucide:sparkles" />
      <span class="global-effect-toolbar__count" aria-hidden="true">{{ effectCountLabel }}</span>
    </button>

    <el-drawer
      v-if="isMobile"
      v-model="drawerVisible"
      direction="btt"
      size="min(64vh, 520px)"
      :title="t('game.operationEffects.title')"
      append-to-body
      class="global-effect-toolbar-drawer"
    >
      <OperationEffectList :items="activeEffects" :now-ms="nowMs" />
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { useDevice } from "@/composables/game/useDevice"
import { useOperationEffects } from "@/composables/game/useOperationEffects"
import OperationEffectList from "./OperationEffectList.vue"

const { t } = useI18n()
const { isMobile } = useDevice()
const { activeEffects, serverNowMs: nowMs } = useOperationEffects()
const popoverVisible = ref(false)
const drawerVisible = ref(false)

const effectCountLabel = computed(() => activeEffects.value.length > 99 ? "99+" : activeEffects.value.length)

watch(activeEffects, (items) => {
  if (items.length > 0) return
  popoverVisible.value = false
  drawerVisible.value = false
})
watch(isMobile, () => {
  popoverVisible.value = false
  drawerVisible.value = false
})
</script>

<style scoped>
.global-effect-toolbar {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
}

.global-effect-toolbar__trigger {
  position: relative;
  display: inline-flex;
  width: 36px;
  height: 36px;
  cursor: pointer;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 1px solid var(--app-border);
  border-radius: 50%;
  background: var(--app-surface);
  color: var(--app-primary);
  font-size: 18px;
  transition: border-color 160ms ease, background-color 160ms ease, transform 160ms ease;
}

.global-effect-toolbar__trigger:hover,
.global-effect-toolbar__trigger:focus-visible {
  border-color: var(--app-primary);
  background: color-mix(in srgb, var(--app-primary) 8%, var(--app-surface));
}

.global-effect-toolbar__trigger:active {
  transform: translateY(1px);
}

.global-effect-toolbar__trigger:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--app-primary) 50%, transparent);
  outline-offset: 2px;
}

.global-effect-toolbar__count {
  position: absolute;
  top: -6px;
  right: -6px;
  display: grid;
  place-items: center;
  box-sizing: border-box;
  width: 20px;
  height: 20px;
  padding: 0;
  border: 2px solid var(--app-surface);
  border-radius: 50%;
  background: var(--app-primary);
  color: #fff;
  font-size: 9px;
  font-weight: 650;
  line-height: 1;
  font-variant-numeric: tabular-nums;
}

.global-effect-toolbar__panel {
  color: var(--app-text);
}

.global-effect-toolbar__heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 1px 2px 10px;
  border-bottom: 1px solid var(--app-border);
  font-size: 14px;
  font-weight: 650;
}

.global-effect-toolbar__heading strong {
  color: var(--app-primary);
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

:global(.global-effect-toolbar-popper.el-popover) {
  border-color: var(--app-border);
  border-radius: 8px;
  background: var(--app-surface-elevated);
  box-shadow: 0 12px 30px rgb(0 0 0 / 14%);
}

:global(.global-effect-toolbar-drawer.el-drawer) {
  border-radius: 8px 8px 0 0;
  background: var(--app-surface-elevated);
}

:global(.global-effect-toolbar-drawer .el-drawer__header) {
  margin-bottom: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid var(--app-border);
  color: var(--app-text);
  font-size: 15px;
  font-weight: 650;
}

:global(.global-effect-toolbar-drawer .el-drawer__body) {
  padding: 12px 18px max(18px, env(safe-area-inset-bottom));
}
</style>
