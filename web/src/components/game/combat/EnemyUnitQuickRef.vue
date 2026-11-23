<template>
  <el-popover
    v-model:visible="popoverActive"
    :width="220"
    :trigger="supportsHover ? 'hover' : 'manual'"
    :persistent="false"
    :show-after="supportsHover ? 180 : 0"
    @show="onPopoverShow"
    @hide="onPopoverHide"
  >
    <template #reference>
      <el-tag
        class="enemy-unit-quick-ref-tag"
        effect="plain"
        :disable-transitions="true"
        @mouseenter="primePopover"
        @focusin="primePopover"
        @click.stop="openDetail"
      >
        {{ enemyName }}
      </el-tag>
    </template>
    <div class="text-xs app-muted">{{ t("game.battleEnemyQuickRefHint") }}</div>
  </el-popover>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { useI18n } from "vue-i18n"
import { useDevice } from "@composables/game/useDevice"
import { getLocalizedEnemyName } from "@services/enemy-definition"
import {
  useEnemyUnitQuickRefDialogStore,
  type EnemyQuickRefDifficultyContext
} from "@stores/enemy-unit-quick-ref-dialog"

const props = defineProps<{
  enemyId: string
  difficultyContext?: EnemyQuickRefDifficultyContext | null
}>()

const { t } = useI18n()
const { supportsHover } = useDevice()
const dialogStore = useEnemyUnitQuickRefDialogStore()
const popoverActive = ref(false)
const enemyName = computed(() => getLocalizedEnemyName(t, props.enemyId, props.enemyId))

function openDetail() {
  dialogStore.open({
    enemyId: props.enemyId,
    difficultyContext: props.difficultyContext
  })
}

function onPopoverShow() {
  if (!supportsHover.value) return
  popoverActive.value = true
}

function onPopoverHide() {
  popoverActive.value = false
}

function primePopover() {
  if (!supportsHover.value) return
}
</script>

<style scoped>
.enemy-unit-quick-ref-tag {
  cursor: pointer;
}
</style>
