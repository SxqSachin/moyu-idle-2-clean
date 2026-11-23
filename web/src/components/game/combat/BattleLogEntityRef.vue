<template>
  <SkillQuickRef
    v-if="reference.kind === 'skill'"
    :skill-id="reference.id"
    :show-icon="true"
  />
  <ItemQuickRef
    v-else-if="reference.kind === 'item'"
    :item-id="reference.id"
  />
  <BuffQuickRef
    v-else-if="reference.kind === 'buff'"
    :buff-id="reference.id"
    :remain-turns="reference.remainTurns"
    :payload="reference.payload"
    :stack-count="reference.stackCount"
    :teleported="true"
  />
  <EnemyUnitQuickRef
    v-else-if="reference.kind === 'unit' && reference.enemyId"
    :enemy-id="reference.enemyId"
  />
  <span v-else class="battle-log-entity-ref__label">{{ reference.label || reference.id }}</span>
</template>

<script setup lang="ts">
import SkillQuickRef from "@components/game/skill/SkillQuickRef.vue"
import ItemQuickRef from "@components/game/item/QuickRef.vue"
import BuffQuickRef from "@components/game/buff/BuffQuickRef.vue"
import EnemyUnitQuickRef from "@components/game/combat/EnemyUnitQuickRef.vue"
import type { BattleLogEntityReference } from "@services/battle-log"

defineProps<{
  reference: BattleLogEntityReference
}>()
</script>

<style scoped>
.battle-log-entity-ref__label {
  display: inline;
  color: var(--el-text-color-primary);
  font-weight: 600;
}

:deep(.el-tag) {
  max-width: min(100%, 260px);
  height: 24px;
  vertical-align: middle;
}

:deep(.el-tag__content),
:deep(.item-quick-ref),
:deep(.item-quick-ref-name) {
  min-width: 0;
  max-width: 100%;
}

:deep(.item-quick-ref-name) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:deep(.item-quick-ref svg),
:deep(.item-quick-ref img) {
  width: 16px !important;
  height: 16px !important;
}
</style>
