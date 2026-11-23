<template>
  <div v-if="battleUsableItem" class="item-popover-row">
    <div class="item-popover-value battle-item-quick-ref-content detail-surface">
      <template v-if="battleUsableItem.isTrigger">
        <div class="battle-item-quick-ref-section-title detail-section-title">{{ t("game.triggerItemInfo") }}</div>
        <div class="battle-item-quick-ref-section detail-section grid-cols-2">
          <div v-if="battleUsableItem.dungeonIds?.length" class="flex flex-row flex-wrap gap-2 items-center">
            <div class="battle-item-quick-ref-label detail-row-label">{{ t("game.triggerItemDungeons") }}:</div>
            <div class="battle-item-quick-ref-value detail-row-value">{{ dungeonNames }}</div>
          </div>
          <div v-if="battleUsableItem.spawnEnemyIds?.length" class="flex flex-row flex-wrap gap-2 items-center">
            <div class="battle-item-quick-ref-label detail-row-label">{{ t("game.triggerItemEnemies") }}:</div>
            <div class="battle-item-quick-ref-value detail-row-value">{{ enemyNames }}</div>
          </div>
          <div v-if="battleUsableItem.minWave !== undefined" class="flex flex-row flex-wrap gap-2 items-center">
            <div class="battle-item-quick-ref-label detail-row-label">{{ t("game.triggerItemMinWave") }}:</div>
            <div class="battle-item-quick-ref-value detail-row-value">{{ battleUsableItem.minWave }}</div>
          </div>
          <div v-if="battleUsableItem.intervalWaves !== undefined && battleUsableItem.intervalWaves > 0" class="flex flex-row flex-wrap gap-2 items-center">
            <div class="battle-item-quick-ref-label detail-row-label">{{ t("game.triggerItemIntervalWaves") }}:</div>
            <div class="battle-item-quick-ref-value detail-row-value">{{ battleUsableItem.intervalWaves }}</div>
          </div>
          <div v-if="battleUsableItem.perWaveChance !== undefined" class="flex flex-row flex-wrap gap-2 items-center">
            <div class="battle-item-quick-ref-label detail-row-label">{{ t("game.triggerItemChance") }}:</div>
            <div class="battle-item-quick-ref-value detail-row-value">{{ perWaveChancePct }}</div>
          </div>
          <div v-if="battleUsableItem.partyTriggerGroup" class="flex flex-row flex-wrap gap-2 items-center">
            <div class="battle-item-quick-ref-label detail-row-label">{{ t("game.triggerItemPartyGroup") }}:</div>
            <div class="battle-item-quick-ref-value detail-row-value">{{ t("game.triggerItemPartyGroupValue") }}</div>
          </div>
        </div>
        <div class="trigger-item-notes">
          <div class="trigger-item-note">{{ t("game.triggerItemBagNote") }}</div>
          <div class="trigger-item-note">{{ t("game.triggerItemConsumeNote") }}</div>
        </div>
      </template>

      <template v-if="!battleUsableItem.isTrigger">
        <div class="battle-item-quick-ref-section-title detail-section-title">{{ t("game.skillBaseInfo") }}</div>
        <div class="battle-item-quick-ref-section detail-section grid-cols-2">
          <div class="flex flex-row flex-wrap gap-2 items-center">
            <div class="battle-item-quick-ref-label detail-row-label">{{ t("game.battleUnitCooldown") }}:</div>
            <div class="battle-item-quick-ref-value detail-row-value">{{ battleUsableItem.cooldownTurns }}</div>
          </div>
        </div>

        <div v-for="section in effectSections" :key="`battle-usable-item-${battleUsableItem.id}-${section.key}`">
          <div v-if="section.title" class="battle-item-quick-ref-section-title detail-section-title">{{ section.title }}</div>
          <div class="battle-item-quick-ref-section detail-section grid-cols-2">
            <div
              v-for="row in section.rows"
              :key="`battle-usable-item-${battleUsableItem.id}-${section.key}-${row.label}`"
              class="flex flex-row flex-wrap gap-2 items-center"
            >
              <div class="battle-item-quick-ref-label detail-row-label">
                {{ row.label }}:
                <el-tooltip v-if="row.labelHint" :content="row.labelHint" placement="top">
                  <el-icon class="battle-item-quick-ref-label-hint detail-row-hint"><QuestionFilled /></el-icon>
                </el-tooltip>
              </div>
              <div class="battle-item-quick-ref-value detail-row-value">
                <div v-if="row.buffIds?.length" class="battle-item-quick-ref-buff-list detail-buff-list">
                  <BuffQuickRef
                    v-for="buffId in row.buffIds"
                    :key="`battle-usable-item-buff-${battleUsableItem.id}-${buffId}`"
                    :buff-id="buffId"
                    :remain-turns="-1"
                  />
                </div>
                <template v-else>
                  <span>{{ row.value }}</span>
                  <span v-if="row.preview" class="battle-item-quick-ref-value-preview detail-row-preview"> {{ row.preview }}</span>
                </template>
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { QuestionFilled } from "@element-plus/icons-vue"
import type { BattleUsableItemDefinition } from "@/types/equipment"
import BuffQuickRef from "@components/game/buff/BuffQuickRef.vue"
import { buildEffectDetailSections } from "@services/effect-detail-renderer"

const props = defineProps<{
  battleUsableItem?: BattleUsableItemDefinition
}>()

const { t, te } = useI18n()

const effectSections = computed(() => {
  if (!props.battleUsableItem) return []
  return buildEffectDetailSections({
    effects: props.battleUsableItem.effects,
    t,
    variant: "battle_item"
  })
})

const dungeonNames = computed(() => {
  const ids = props.battleUsableItem?.dungeonIds ?? []
  return ids
    .map((id) => {
      const key = `world.${id}.name`
      return te(key) ? t(key) : id
    })
    .join(" / ")
})

const enemyNames = computed(() => {
  const ids = props.battleUsableItem?.spawnEnemyIds ?? []
  return ids
    .map((id) => {
      const key = `enemy.${id}.name`
      return te(key) ? t(key) : id
    })
    .join(" / ")
})

const perWaveChancePct = computed(() => {
  const c = props.battleUsableItem?.perWaveChance
  if (c === undefined) return ""
  return `${Math.round(c * 100)}%`
})
</script>

<style scoped>
@reference "#app-style";

.battle-item-quick-ref-content {
  display: grid;
  gap: 8px;
}

.battle-item-quick-ref-section { display: grid; }

.trigger-item-notes {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.trigger-item-note {
  font-size: 0.75rem;
  color: var(--app-muted);
  font-style: italic;
}
</style>
