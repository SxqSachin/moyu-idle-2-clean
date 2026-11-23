<template>
  <div class="page-shell space-y-4">
    <el-card class="app-card app-card--no-border game-page-frame" shadow="never" data-tutorial="affix-page">
      <template #header>
        <GamePageHeader
          title-key="game.enchant"
          tutorial-id="affix"
          :visual-src="affixVisual.src"
          :visual-tone="affixVisual.tone"
          visual-key="affix"
        >
          <template #actions>
            <el-button size="small" @click="refreshAll">{{ t("game.refresh") }}</el-button>
          </template>
        </GamePageHeader>
      </template>

      <div class="space-y-4">
        <div class="space-y-2" data-tutorial="affix-target">
          <div class="text-sm">{{ t("game.affixTargetEquipment") }}</div>
          <div class="flex flex-wrap items-center gap-2">
            <Card
              v-if="selectedEquipmentEntry"
              :item-id="selectedEquipmentEntry.templateItemId"
              :instance-id="selectedEquipmentEntry.instanceId"
              :quantity="selectedEquipmentEntry.quantity"
              @click="equipmentSelectorVisible = true"
            />
            <EmptyCard v-else :label="t('game.inventorySelectorChoose')" @click="equipmentSelectorVisible = true" />
            <div class="text-xs app-muted" v-if="equipmentInstanceIds.length > 0">
              {{ t("game.itemDetailOwnedCount", { count: equipmentInstanceIds.length }) }}
            </div>
            <el-alert
              v-if="detachDestroyedNotice"
              class="min-w-[260px] max-w-[520px]"
              :title="t('game.affixDetachEquipmentDestroyed')"
              type="warning"
              show-icon
              @close="detachDestroyedNotice = false"
            />
          </div>
        </div>

        <el-tabs v-model="activeTab" @tab-change="handleTabChange" data-tutorial="affix-tabs">
          <el-tab-pane :label="t('game.enchant')" name="enchant" />
          <el-tab-pane :label="t('game.itemDetailDetachAffix')" name="detach" />
          <el-tab-pane :label="t('game.itemDetailAttachAffix')" name="attach" />
        </el-tabs>

        <div v-if="!selectedEquipmentInstanceId" class="text-sm app-muted py-2">
          {{ t("game.equipmentOperationTargetMissing") }}
        </div>

        <div v-else v-loading="loading" data-tutorial="affix-operation">
          <AffixEnchantPanel
            v-if="activeTab === 'enchant'"
            :detail="detail"
            :target-instance-id="selectedEquipmentInstanceId"
            @submitted="handleSubmitted"
          />
          <AffixDetachPanel
            v-else-if="activeTab === 'detach'"
            :detail="detail"
            :target-instance-id="selectedEquipmentInstanceId"
            @submitted="handleSubmitted"
          />
          <AffixAttachPanel
            v-else
            :detail="detail"
            :target-instance-id="selectedEquipmentInstanceId"
            @submitted="handleSubmitted"
          />
        </div>
      </div>
    </el-card>

    <InventoryInstanceSelectorDialog
      v-model="equipmentSelectorVisible"
      :title="t('game.affixTargetEquipment')"
      :instance-ids="equipmentInstanceIds"
      :selected-instance-id="selectedEquipmentInstanceId"
      :allow-clear="true"
      :template-item-ids="equipmentTemplateItemIds"
      :template-items-title="t('game.inventorySelectorAvailableItemsTitle', { title: t('game.affixTargetEquipment') })"
      :empty-text="t('game.inventorySelectorNoItems')"
      @confirm="handleSelectEquipment"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { useRoute, useRouter } from "vue-router"
import { resolveErrorMessage } from "@services/error-messages"
import { hasAffixCapacity } from "@services/equipment-affix-slots"
import { getEquipmentItemDefinition, getItemDefinition } from "@services/item-definition"
import { useGameStore } from "@stores/game"
import { useInventoryStore } from "@stores/inventory"
import { useEquipmentStore } from "@stores/equipment"
import { useItemInstanceDetailStore } from "@stores/item-instance-detail"
import AffixEnchantPanel from "@components/game/equipment-affix/AffixEnchantPanel.vue"
import AffixDetachPanel from "@components/game/equipment-affix/AffixDetachPanel.vue"
import AffixAttachPanel from "@components/game/equipment-affix/AffixAttachPanel.vue"
import Card from "@components/game/item/Card.vue"
import EmptyCard from "@components/game/item/EmptyCard.vue"
import InventoryInstanceSelectorDialog from "@components/game/inventory/InventoryInstanceSelectorDialog.vue"
import GamePageHeader from "@components/game/common/GamePageHeader.vue"
import { gamePageVisuals } from "@services/game-page-visuals"
import type { ItemDetailResponse } from "@/types/equipment"

type AffixTab = "enchant" | "detach" | "attach"

const { t } = useI18n()
const affixVisual = gamePageVisuals.affix
const route = useRoute()
const router = useRouter()
const gameStore = useGameStore()
const inventoryStore = useInventoryStore()
const equipmentStore = useEquipmentStore()
const instanceDetailStore = useItemInstanceDetailStore()

const loading = ref(false)
const detail = ref<ItemDetailResponse | null>(null)
const activeTab = ref<AffixTab>("enchant")
const selectedEquipmentInstanceId = ref<number | null>(null)
const equipmentSelectorVisible = ref(false)
const detachDestroyedNotice = ref(false)

const inventoryByInstanceId = computed(() => {
  const map = new Map<number, (typeof inventoryStore.instances)[number]>()
  for (const item of inventoryStore.instances) {
    map.set(item.instanceId, item)
  }
  return map
})

const equipmentInstanceIds = computed(() =>
  inventoryStore.instances
    .filter(
      (item) =>
        item.state === "inventory" &&
        item.quantity > 0 &&
        getItemDefinition(item.templateItemId)?.isEquipment &&
        hasAffixCapacity(getEquipmentItemDefinition(item.templateItemId)?.affixSlotCount || 0)
    )
    .map((item) => item.instanceId)
)
const equipmentTemplateItemIds = computed(() => {
  const ids: string[] = []
  const seen = new Set<string>()
  for (const item of inventoryStore.instances) {
    if (item.state !== "inventory" || item.quantity <= 0) continue
    if (!getItemDefinition(item.templateItemId)?.isEquipment) continue
    if (!hasAffixCapacity(getEquipmentItemDefinition(item.templateItemId)?.affixSlotCount || 0)) continue
    const id = String(item.templateItemId || "").trim()
    if (!id || seen.has(id)) continue
    seen.add(id)
    ids.push(id)
  }
  return ids
})
const selectedEquipmentEntry = computed(() =>
  inventoryStore.instances.find((item) => item.instanceId === selectedEquipmentInstanceId.value) || null
)

onMounted(() => {
  void initialize()
})

watch(
  () => route.query,
  () => {
    syncStateFromRoute()
  }
)

watch(
  () => selectedEquipmentInstanceId.value,
  async (instanceId) => {
    if (!instanceId) {
      detail.value = null
      return
    }
    await loadDetail(instanceId)
    syncRouteQuery()
  }
)

async function initialize() {
  gameStore.restoreToken()
  if (!gameStore.token) return
  await inventoryStore.initialize(gameStore.token)
  syncStateFromRoute()
  if (!selectedEquipmentInstanceId.value) {
    const operationTargetInstanceId = equipmentStore.operationTarget?.instanceId || null
    const fallback = operationTargetInstanceId && equipmentInstanceIds.value.includes(operationTargetInstanceId)
      ? operationTargetInstanceId
      : equipmentInstanceIds.value[0] || null
    selectedEquipmentInstanceId.value = fallback
  }
  if (selectedEquipmentInstanceId.value) {
    await loadDetail(selectedEquipmentInstanceId.value)
  }
}

function syncStateFromRoute() {
  const tabRaw = typeof route.query.tab === "string" ? route.query.tab : ""
  if (tabRaw === "smelt") {
    void router.replace({ path: "/game/alchemy", query: { tab: "smelt" } })
    return
  }
  if (tabRaw === "detach" || tabRaw === "attach" || tabRaw === "enchant") {
    activeTab.value = tabRaw
  }
}

function syncRouteQuery() {
  const query = {
    ...route.query,
    tab: activeTab.value
  }
  void router.replace({ path: "/game/affix", query })
}

async function loadDetail(instanceId: number) {
  if (!gameStore.token || instanceId <= 0) return
  loading.value = true
  try {
    detail.value = await instanceDetailStore.fetchByInstance(gameStore.token, instanceId, { force: true })
  } catch (err) {
    detail.value = null
    ElMessage.error(resolveErrorMessage(t, err, "game.itemDetailLoadFailed"))
  } finally {
    loading.value = false
  }
}

async function refreshAll() {
  if (!gameStore.token) return
  await inventoryStore.initialize(gameStore.token, true)
  if (selectedEquipmentInstanceId.value) {
    await loadDetail(selectedEquipmentInstanceId.value)
  }
}

function handleTabChange(name: string | number) {
  if (name === "enchant" || name === "detach" || name === "attach") {
    activeTab.value = name
    syncRouteQuery()
  }
}

function handleSelectEquipment(instanceId: number | null) {
  detachDestroyedNotice.value = false
  selectedEquipmentInstanceId.value = instanceId
  if (!instanceId) return
  const instance = inventoryByInstanceId.value.get(instanceId)
  if (instance) {
    equipmentStore.setOperationTarget({
      instanceId: instance.instanceId,
      templateItemId: instance.templateItemId
    })
  }
}

async function handleSubmitted(result?: { equipmentInstanceId?: number; destroyed?: boolean }) {
  try {
    if (result?.destroyed) {
      detachDestroyedNotice.value = true
      if (gameStore.token) {
        await inventoryStore.initialize(gameStore.token, true)
      }
      if (typeof result.equipmentInstanceId === "number") {
        instanceDetailStore.invalidate(result.equipmentInstanceId)
      }
      if (selectedEquipmentInstanceId.value === result.equipmentInstanceId) {
        selectedEquipmentInstanceId.value = null
        detail.value = null
        equipmentStore.clearOperationTarget()
      }
      return
    }
    detachDestroyedNotice.value = false
    await refreshAll()
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.itemOperationFailed"))
  }
}

</script>
