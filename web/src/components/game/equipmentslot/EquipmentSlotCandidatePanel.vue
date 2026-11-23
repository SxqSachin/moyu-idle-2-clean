<template>
  <div class="slot-dropdown-panel">
    <el-tabs
      v-if="archiveTabsVisible"
      v-model="activeArchiveTab"
      class="slot-dropdown-archive-tabs"
    >
      <el-tab-pane :label="t('game.inventoryTab')" name="inventory" />
      <el-tab-pane :label="t('game.inventoryArchivedTab')" name="archived" />
    </el-tabs>

    <div v-if="visibleEligibleCandidates.length === 0 && visibleIneligibleCandidates.length === 0" class="text-sm app-muted">
      {{ t("game.equipmentNoCandidate") }}
    </div>
    <div v-else class="slot-dropdown-stack">
      <section v-if="visibleEligibleCandidates.length > 0" class="slot-dropdown-column">
        <div class="slot-dropdown-title">{{ t("game.equipmentCandidateEligible") }}</div>
        <div class="slot-dropdown-sections">
          <section v-for="section in groupedEligibleSections" :key="`eligible-${section.id}`" class="slot-dropdown-sub-section">
            <div class="slot-dropdown-sub-title">{{ section.label }}</div>
            <div class="slot-dropdown-items-grid">
              <EquipmentSlotCandidateCard
                v-for="candidate in section.items"
                :key="candidate.instanceId"
                :candidate="candidate"
                :equipping-instance-id="equippingInstanceId"
                @equip="$emit('equip', $event)"
              />
            </div>
          </section>
        </div>
      </section>
      <section v-if="visibleIneligibleCandidates.length > 0" class="slot-dropdown-column">
        <div class="slot-dropdown-title">{{ t("game.equipmentCandidateIneligible") }}</div>
        <div class="slot-dropdown-sections">
          <section v-for="section in groupedIneligibleSections" :key="`ineligible-${section.id}`" class="slot-dropdown-sub-section">
            <div class="slot-dropdown-sub-title">{{ section.label }}</div>
            <div class="slot-dropdown-items-grid">
              <EquipmentSlotCandidateCard
                v-for="candidate in section.items"
                :key="candidate.instanceId"
                :candidate="candidate"
                :equipping-instance-id="equippingInstanceId"
                @equip="$emit('equip', $event)"
              />
            </div>
          </section>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import type { SlotCandidate } from "@/composables/game/attributes/useEquipmentSlots"
import { buildEquipmentSlotSecondarySections } from "@/services/equipment-grouping"
import EquipmentSlotCandidateCard from "@components/game/equipmentslot/EquipmentSlotCandidateCard.vue"

const props = defineProps<{
  eligibleCandidates: SlotCandidate[]
  ineligibleCandidates: SlotCandidate[]
  equippingInstanceId: number | null
}>()

defineEmits<{
  (e: "equip", instanceID: number): void
}>()

const { t } = useI18n()
const activeArchiveTab = ref<"inventory" | "archived">("inventory")

const archivedCandidateCount = computed(() =>
  [...props.eligibleCandidates, ...props.ineligibleCandidates].filter((candidate) => !!candidate.archivedAt).length
)
const archiveTabsVisible = computed(() => archivedCandidateCount.value > 0)
const visibleEligibleCandidates = computed(() =>
  props.eligibleCandidates.filter((candidate) => activeArchiveTab.value === "archived" ? !!candidate.archivedAt : !candidate.archivedAt)
)
const visibleIneligibleCandidates = computed(() =>
  props.ineligibleCandidates.filter((candidate) => activeArchiveTab.value === "archived" ? !!candidate.archivedAt : !candidate.archivedAt)
)

const groupedEligibleSections = computed(() =>
  buildEquipmentSlotSecondarySections({
    items: visibleEligibleCandidates.value,
    getItemId: (candidate) => candidate.templateItemId,
    t
  }).flatMap((slotSection) => slotSection.subSections || [{ id: `${slotSection.id}-all`, label: slotSection.label, items: slotSection.items }])
)

const groupedIneligibleSections = computed(() =>
  buildEquipmentSlotSecondarySections({
    items: visibleIneligibleCandidates.value,
    getItemId: (candidate) => candidate.templateItemId,
    t
  }).flatMap((slotSection) => slotSection.subSections || [{ id: `${slotSection.id}-all`, label: slotSection.label, items: slotSection.items }])
)

watch(
  () => [props.eligibleCandidates, props.ineligibleCandidates],
  () => {
    activeArchiveTab.value = "inventory"
  }
)
</script>

<style scoped>
.slot-dropdown-panel {
  overflow: auto;
  padding-right: 2px;
}

.slot-dropdown-archive-tabs {
  margin-bottom: 8px;
}

.slot-dropdown-archive-tabs :deep(.el-tabs__content) {
  display: none;
}

.slot-dropdown-archive-tabs :deep(.el-tabs__header) {
  margin: 0;
}

.slot-dropdown-stack {
  display: grid;
  gap: 12px;
}

.slot-dropdown-column {
  display: grid;
  gap: 8px;
  align-content: start;
}

.slot-dropdown-sections {
  display: grid;
  gap: 8px;
}

.slot-dropdown-sub-section {
  display: grid;
  gap: 6px;
}

.slot-dropdown-sub-title {
  font-size: 11px;
  color: var(--app-text-muted, var(--el-text-color-secondary));
}

.slot-dropdown-items-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(64px, 1fr));
  gap: 8px;
}

@media (max-width: 767px) {
  .slot-dropdown-items-grid {
    grid-template-columns: repeat(auto-fill, minmax(56px, 1fr));
  }
}

.slot-dropdown-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--app-muted);
}

</style>
