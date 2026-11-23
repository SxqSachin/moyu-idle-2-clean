<template>
  <div class="space-y-3">
    <div v-if="!kittyId" class="rounded border border-dashed border-[var(--app-border)] p-4 text-sm app-muted">
      请选择一只猫咪查看战斗属性、技能与装备。
    </div>
    <div v-else-if="combatLoading" class="rounded border border-[var(--app-border)] p-4 text-sm app-muted">加载中...</div>
    <div v-else-if="combatReady" class="space-y-3">
      <!-- <div v-if="kittyBattleLevelCapText" class="rounded border border-[var(--app-border)] bg-[var(--app-surface-elevated)] px-3 py-2 text-xs app-muted">
        {{ kittyBattleLevelCapText }}
      </div> -->
      <el-tabs v-if="embeddedTabs" :model-value="activeTab" @update:model-value="emit('update:activeTab', String($event || 'attributes'))">
        <el-tab-pane label="属性" name="attributes">
          <section v-if="kittyPersonalityIds.length > 0" class="rounded-lg border border-[var(--app-border)] p-3">
            <div class="mb-2 text-sm font-semibold">
              {{ t("game.kittyTraitTitle") }}
              <span class="ml-1 text-xs app-muted">{{ kittyPersonalityIds.length }}/{{ KITTY_MAX_PERSONALITIES }}</span>
            </div>
            <div class="flex flex-wrap gap-2">
              <KittyPersonalityQuickRef
                v-for="personalityId in kittyPersonalityIds"
                :key="`combat-personality-${kittyId}-${personalityId}`"
                :personality-id="personalityId"
              />
            </div>
          </section>
          <RoleStatsPanel
            :role-key="combatRoleKey"
            :row-filter="combatAttributeRowFilter"
            :visible-group-keys="['battle', 'work']"
            :split-work-sections-into-two-columns="true"
            @refresh="loadCombatProfile"
            @open-allocate-dialog="openAllocateDialog"
          />
        </el-tab-pane>
        <el-tab-pane label="技能" name="skills">
          <AttributesSkillTab
            v-if="token && kittyId > 0"
            :token="token"
            :config-get-path="kittyCombatConfigPath"
            :config-post-path="kittyCombatConfigPath"
            :rule-post-path="kittyRuleConfigPath"
            :unit-attributes="combatUnitAttributes"
          />
        </el-tab-pane>
      <el-tab-pane label="装备" name="equipment">
          <EquipmentSlotTab
            :slot-cards="kittySlotCards"
            :equipment-type-active-tab="kittyEquipmentTypeActiveTab"
            :equipping-instance-id="equippingInstanceId"
            :equip-success-version="equipSuccessVersion"
            :allowed-equip-types="['battle', 'fishing']"
            @equip="equipKittyToSlot"
            @updated="refreshCombatEquipment"
            @update:equipment-type-active-tab="kittyEquipmentTypeActiveTab = $event"
          >
            <template #occupied-detail="{ instance, close }">
              <ItemDetailPanel
                :item-id="String(instance.templateItemId || '')"
                :instance-id="Number(instance.instanceId || 0)"
                :active="true"
                operation-target-type="kitty"
                :operation-target-kitty-id="kittyId"
                @updated="refreshCombatEquipment"
                @request-close="close"
              />
            </template>
          </EquipmentSlotTab>
        </el-tab-pane>
      </el-tabs>
      <template v-else-if="activeTab === 'attributes'">
        <section v-if="kittyPersonalityIds.length > 0" class="rounded-lg border border-[var(--app-border)] p-3">
          <div class="mb-2 text-sm font-semibold">
            {{ t("game.kittyTraitTitle") }}
            <span class="ml-1 text-xs app-muted">{{ kittyPersonalityIds.length }}/{{ KITTY_MAX_PERSONALITIES }}</span>
          </div>
          <div class="flex flex-wrap gap-2">
            <KittyPersonalityQuickRef
              v-for="personalityId in kittyPersonalityIds"
              :key="`combat-personality-${kittyId}-${personalityId}`"
              :personality-id="personalityId"
            />
          </div>
        </section>
        <RoleStatsPanel
          :role-key="combatRoleKey"
          :row-filter="combatAttributeRowFilter"
          :visible-group-keys="['battle', 'work']"
          :split-work-sections-into-two-columns="true"
          @refresh="loadCombatProfile"
          @open-allocate-dialog="openAllocateDialog"
        />
      </template>
      <AttributesSkillTab
        v-else-if="activeTab === 'skills' && token && kittyId > 0"
        :token="token"
        :config-get-path="kittyCombatConfigPath"
        :config-post-path="kittyCombatConfigPath"
        :rule-post-path="kittyRuleConfigPath"
        :unit-attributes="combatUnitAttributes"
      />
      <EquipmentSlotTab
        v-else-if="activeTab === 'equipment'"
        :slot-cards="kittySlotCards"
        :equipment-type-active-tab="kittyEquipmentTypeActiveTab"
        :equipping-instance-id="equippingInstanceId"
        :equip-success-version="equipSuccessVersion"
        :allowed-equip-types="['battle', 'fishing']"
        @equip="equipKittyToSlot"
        @updated="refreshCombatEquipment"
        @update:equipment-type-active-tab="kittyEquipmentTypeActiveTab = $event"
      >
        <template #occupied-detail="{ instance, close }">
          <ItemDetailPanel
            :item-id="String(instance.templateItemId || '')"
            :instance-id="Number(instance.instanceId || 0)"
            :active="true"
            operation-target-type="kitty"
            :operation-target-kitty-id="kittyId"
            @updated="refreshCombatEquipment"
            @request-close="close"
          />
        </template>
      </EquipmentSlotTab>

      <GameDialog v-model="allocateDialogVisible" :title="t('game.battlePointAllocateBatchTitle')" width="520px">
        <div class="space-y-3">
          <div class="rounded-lg border border-[var(--app-border)] p-3">
            <div class="mb-1 text-sm font-medium">{{ t("game.battlePointTitle") }}</div>
            <div class="mb-2 text-xs app-muted">
              {{ t("game.battlePointSummary", { total: battlePointState.totalPoints, spent: battlePointState.spentPoints, available: battlePointState.available }) }}
            </div>
            <div class="flex flex-wrap items-center gap-2 text-xs">
              <QuickRefWithRequire
                v-if="battlePointState.resetRequirement.itemId"
                :item-id="battlePointState.resetRequirement.itemId"
                :required-count="battlePointState.resetRequirement.count"
              />
              <el-button :disabled="battlePointState.spentPoints <= 0 || resetSubmitting" :loading="resetSubmitting" @click="resetBattlePoints">
                {{ t("game.battlePointReset") }}
              </el-button>
            </div>
          </div>
          <div class="text-xs app-muted">
            {{ t("game.battlePointAllocateBatchSummary", { available: battlePointState.available, assigned: allocateBatchTotal, remaining: allocateBatchRemaining }) }}
          </div>
          <div class="space-y-2">
            <div
              v-for="row in allocatableRows"
              :key="`allocate-${row.key}`"
              class="flex items-center justify-between gap-3 rounded border border-[var(--app-border)] px-3 py-2"
            >
              <div class="flex items-center gap-2 text-sm">
                <span>{{ row.name }} · {{ row.value }}</span>
                <el-popover placement="top" trigger="hover" :width="220">
                  <template #reference>
                    <button
                      type="button"
                      class="inline-flex h-4 w-4 items-center justify-center rounded-full border border-white/20 text-[10px] leading-none opacity-80 transition hover:opacity-100"
                      aria-label="Attribute per-point effect details"
                    >
                      <el-icon>
                        <Icon icon="ep:question-filled" />
                      </el-icon>
                    </button>
                  </template>
                  <div class="space-y-1 text-xs whitespace-pre-wrap">
                    <div class="font-semibold">{{ t("game.attrPerPointImpactTitle") }}</div>
                    <div>{{ row.impactLines.length > 0 ? row.impactLines.join("\n") : "-" }}</div>
                  </div>
                </el-popover>
              </div>
              <el-input-number
                :model-value="allocateBatchValues[row.key] || 0"
                :min="0"
                :max="allocationMaxFor(row.key)"
                :step="1"
                @update:model-value="handleBatchAllocationInput(row.key, $event)"
              />
            </div>
          </div>
        </div>
        <template #footer>
          <el-button @click="closeAllocateDialog">{{ t("game.cancel") }}</el-button>
          <el-button
            type="primary"
            :disabled="allocateSubmitting || allocateBatchTotal <= 0 || allocateBatchTotal > battlePointState.available"
            :loading="allocateSubmitting"
            @click="submitAllocateBatch"
          >
            {{ t("game.battlePointAllocateSubmit") }}
          </el-button>
        </template>
      </GameDialog>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, toRef } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { useKittyCombatProfile } from "@/composables/game/cattery/useKittyCombatProfile"
import type { AttributeRow } from "@/composables/game/attributes/useAttributeRows"
import { isActionEfficiencyAttributeKey } from "@/services/action-duration"
import { KITTY_MAX_PERSONALITIES } from "@/config/shared-global-config"
import RoleStatsPanel from "@components/game/roles/RoleStatsPanel.vue"
import AttributesSkillTab from "@components/game/attributes/AttributesSkillTab.vue"
import EquipmentSlotTab from "@components/game/equipmentslot/EquipmentSlotTab.vue"
import QuickRefWithRequire from "@components/game/item/QuickRefWithRequire.vue"
import ItemDetailPanel from "@components/game/item/ItemDetailPanel.vue"
import KittyPersonalityQuickRef from "@components/game/cattery/KittyPersonalityQuickRef.vue"
import GameDialog from "@components/game/common/GameDialog.vue"

const props = defineProps<{
  token: string
  kittyId: number
  activeTab: string
  embeddedTabs?: boolean
}>()

const emit = defineEmits<{
  (e: "update:activeTab", value: string): void
}>()

const { t } = useI18n()
const embeddedTabs = computed(() => props.embeddedTabs !== false)
const combatAttributeRowFilter = (row: AttributeRow) =>
  row.category === "battle" ||
  (row.category === "work" &&
    (isActionEfficiencyAttributeKey(row.key) || row.key === "fishing" || row.key.startsWith("fishing_")))
const kittyBattleLevelCapText = computed(() => {
  const kitty = combatProfile.value?.kitty
  const cap = Number(kitty?.battleLevelCap || 0)
  if (!Number.isFinite(cap) || cap <= 0) return ""
  const battleRow = (combatProfile.value?.attributes || []).find((row) => String(row.attrKey || "").trim() === "battle")
  const level = Math.max(1, Math.trunc(Number(battleRow?.baseLevel || 1)))
  return t("game.catteryKittyBattleLevelCapText", { level, cap: Math.trunc(cap) })
})
const kittyPersonalityIds = computed(() => {
  const kitty = combatProfile.value?.kitty
  const extra = kitty?.extra && typeof kitty.extra === "object" ? (kitty.extra as Record<string, unknown>) : null
  const listRaw = extra?.personalityIds
  const out: string[] = []
  if (!Array.isArray(listRaw)) return out
  for (const row of listRaw) {
    const id = String(row || "").trim().toLowerCase()
    if (id && !out.includes(id)) out.push(id)
  }
  return out
})
const {
  combatLoading,
  combatSaving,
  combatProfile,
  combatReady,
  combatRoleKey,
  kittyEquipmentTypeActiveTab,
  equippingInstanceId,
  equipSuccessVersion,
  allocateDialogVisible,
  allocateSubmitting,
  resetSubmitting,
  battlePointState,
  kittyCombatConfigPath,
  kittyRuleConfigPath,
  combatUnitAttributes,
  allocatableRows,
  allocateBatchValues,
  allocateBatchTotal,
  allocateBatchRemaining,
  kittySlotCards,
  loadCombatProfile,
  equipKittyToSlot,
  refreshCombatEquipment,
  openAllocateDialog,
  closeAllocateDialog,
  allocationMaxFor,
  handleBatchAllocationInput,
  submitAllocateBatch,
  resetBattlePoints,
} = useKittyCombatProfile(toRef(props, "token"), toRef(props, "kittyId"), t)
</script>
