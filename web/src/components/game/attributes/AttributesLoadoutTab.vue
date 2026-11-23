<template>
  <div v-loading="loading" class="space-y-5 text-sm" data-tutorial="loadouts-panel">
    <el-tabs v-model="activeTab" type="card" data-tutorial="loadouts-type-tabs">
      <el-tab-pane :label="t('game.loadoutBattleTab')" name="battle">
        <section class="space-y-4" data-tutorial="loadouts-battle">
          <div class="flex items-center justify-between gap-3">
            <div>
              <div class="text-sm font-semibold">{{ t("game.loadoutBattleTab") }}</div>
              <div class="text-xs app-muted">{{ t("game.loadoutBattleSaveHint") }}</div>
            </div>
            <el-button type="primary" @click="openCreateBattleDialog">
              {{ t("game.loadoutCreateBattle") }}
            </el-button>
          </div>

          <div class="grid gap-3 lg:grid-cols-2">
            <el-card v-for="item in loadouts.battleLoadouts" :key="`battle-${item.id}`" class="app-card" shadow="never">
              <div class="space-y-3">
                <div class="flex flex-col items-start justify-between gap-3">
                  <div class="min-w-0">
                    <div class="truncate text-sm font-semibold">{{ item.name }}</div>
                  </div>
                  <div class="flex shrink-0 gap-2">
                    <el-button size="small" type="primary" :loading="applyingBattleId === item.id" @click="applyBattle(item.id)">
                      {{ t("game.loadoutApply") }}
                    </el-button>
                    <el-button size="small" @click="openEditBattleDialog(item)">
                      {{ t("game.loadoutEdit") }}
                    </el-button>
                    <el-button size="small" type="danger" :loading="deletingBattleId === item.id" @click="removeLoadout('battle', item.id)">
                      {{ t("game.loadoutDelete") }}
                    </el-button>
                  </div>
                </div>
              </div>
            </el-card>
            <div v-if="loadouts.battleLoadouts.length <= 0" class="rounded-lg border border-dashed border-[var(--app-border)] px-3 py-6 text-xs app-muted">
              {{ t("game.loadoutEmptyBattle") }}
            </div>
          </div>
        </section>
      </el-tab-pane>

      <el-tab-pane :label="t('game.loadoutWorkTab')" name="work">
        <section class="space-y-4" data-tutorial="loadouts-work">
          <div class="flex items-center justify-between gap-3">
            <div>
              <div class="text-sm font-semibold">{{ t("game.loadoutWorkTab") }}</div>
              <div class="text-xs app-muted">{{ t("game.loadoutWorkSaveHint") }}</div>
            </div>
            <el-button type="primary" @click="openCreateWorkDialog">
              {{ t("game.loadoutCreateWork") }}
            </el-button>
          </div>

          <div class="grid gap-3 lg:grid-cols-2">
            <el-card v-for="item in loadouts.workLoadouts" :key="`work-${item.id}`" class="app-card" shadow="never">
              <div class="space-y-3">
                <div class="flex flex-col items-start justify-between gap-3">
                  <div class="min-w-0">
                    <div class="truncate text-sm font-semibold">{{ item.name }}</div>
                  </div>
                  <div class="flex shrink-0 gap-2">
                    <el-button size="small" type="primary" :loading="applyingWorkId === item.id" @click="applyWork(item.id)">
                      {{ t("game.loadoutApply") }}
                    </el-button>
                    <el-button size="small" @click="openEditWorkDialog(item)">
                      {{ t("game.loadoutEdit") }}
                    </el-button>
                    <el-button size="small" type="danger" :loading="deletingWorkId === item.id" @click="removeLoadout('work', item.id)">
                      {{ t("game.loadoutDelete") }}
                    </el-button>
                  </div>
                </div>
              </div>
            </el-card>
            <div v-if="loadouts.workLoadouts.length <= 0" class="rounded-lg border border-dashed border-[var(--app-border)] px-3 py-6 text-xs app-muted">
              {{ t("game.loadoutEmptyWork") }}
            </div>
          </div>

          <div class="rounded-lg border border-[var(--app-border)] p-3" data-tutorial="loadouts-work-bindings">
            <div class="mb-3 flex items-center justify-between gap-3">
              <div>
                <div class="text-sm font-semibold">{{ t("game.loadoutWorkBindingsTitle") }}</div>
                <div class="text-xs app-muted">{{ t("game.loadoutWorkBindingsHint") }}</div>
              </div>
              <el-button type="primary" :loading="savingBindings" @click="saveBindings">
                {{ t("game.saveConfig") }}
              </el-button>
            </div>
            <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              <div v-for="section in actionSections" :key="`work-binding-${section.id}`" class="rounded border border-[var(--app-border)] px-3 py-2">
                <div class="mb-2 text-sm font-medium">{{ section.label }}</div>
                <el-select v-model="bindingDraft[section.id]" clearable class="w-full" :placeholder="t('game.loadoutNoBinding')">
                  <el-option :label="t('game.loadoutNoBinding')" value="" />
                  <el-option v-for="item in loadouts.workLoadouts" :key="`binding-${section.id}-${item.id}`" :label="item.name || item.id" :value="item.id" />
                </el-select>
              </div>
            </div>
          </div>
        </section>
      </el-tab-pane>
    </el-tabs>

    <GameDialog
      v-model="battleDialogVisible"
      :title="battleDialogTitle"
      :width="battleDialogWidth"
      :fullscreen="isMobile"
      :top="isMobile ? '0' : '6vh'"
      class="loadout-editor-dialog"
    >
      <div class="space-y-4">
        <el-input v-model="battleEditor.name" :placeholder="t('game.loadoutNamePlaceholder')" />
        <el-tabs v-model="battleEditorTab" type="card">
          <el-tab-pane :label="t('game.attributesMainTabEquipment')" name="equipment">
            <LoadoutEquipmentGrid
              :slot-cards="battleDraftSlotCards"
              @equip="handleBattleDraftEquip"
              @unequip="handleBattleDraftUnequip"
            />
          </el-tab-pane>
          <el-tab-pane :label="t('game.attributesMainTabSkills')" name="skills" :lazy="true">
            <AttributesSkillTab
              :token="token"
              mode="draft"
              :draft-skill-slots="battleEditor.skillSlots"
              :draft-skill-rules="battleEditor.skillRules"
              :draft-equipment-skills="battleEditor.equipmentSkills"
              :draft-equipment="battleDraftEquippedItems"
              @update:draft-skill-slots="battleEditor.skillSlots = $event"
              @update:draft-skill-rules="battleEditor.skillRules = $event"
              @update:draft-equipment-skills="battleEditor.equipmentSkills = $event"
            />
          </el-tab-pane>
        </el-tabs>
      </div>
      <template #footer>
        <el-button @click="battleDialogVisible = false">{{ t("game.cancel") }}</el-button>
        <el-button type="primary" :loading="savingBattleEditor" @click="saveBattleEditor">{{ t("game.saveConfig") }}</el-button>
      </template>
    </GameDialog>

    <GameDialog
      v-model="workDialogVisible"
      :title="workDialogTitle"
      :width="workDialogWidth"
      :fullscreen="isMobile"
      :top="isMobile ? '0' : '8vh'"
      class="loadout-editor-dialog"
    >
      <div class="space-y-4">
        <el-input v-model="workEditor.name" :placeholder="t('game.loadoutNamePlaceholder')" />
        <LoadoutEquipmentGrid
          :slot-cards="workDraftSlotCards"
          @equip="handleWorkDraftEquip"
          @unequip="handleWorkDraftUnequip"
        />
      </div>
      <template #footer>
        <el-button @click="workDialogVisible = false">{{ t("game.cancel") }}</el-button>
        <el-button type="primary" :loading="savingWorkEditor" @click="saveWorkEditor">{{ t("game.saveConfig") }}</el-button>
      </template>
    </GameDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, toRaw, watch } from "vue"
import { ElMessage } from "element-plus"
import { useI18n } from "vue-i18n"
import { useEquipmentStore } from "@stores/equipment"
import { useInventoryStore } from "@stores/inventory"
import { useSpecializationStore } from "@stores/specialization"
import { apiGet } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import { listActionMainTypeDefinitions } from "@services/action-definition"
import { getEquipmentTemplateDefinition } from "@services/item-definition"
import { getLocalizedSkillName } from "@services/skill-definition"
import { useDevice } from "@composables/game/useDevice"
import { useEquipmentSlots } from "@/composables/game/attributes/useEquipmentSlots"
import type { InventoryInstance } from "@/types/equipment"
import AttributesSkillTab from "@components/game/attributes/AttributesSkillTab.vue"
import LoadoutEquipmentGrid from "@components/game/equipmentslot/LoadoutEquipmentGrid.vue"
import {
  applyBattleLoadout,
  applyWorkLoadout,
  deletePlayerLoadout,
  fetchPlayerLoadouts,
  type BattleLoadout,
  type LoadoutEquipmentSlot,
  type LoadoutSkillRule,
  type PlayerLoadoutsPayload,
  type WorkLoadout,
  updateWorkLoadoutBindings,
  upsertBattleLoadout,
  upsertWorkLoadout
} from "@services/player-loadouts"
import GameDialog from "@components/game/common/GameDialog.vue"
import type { EquipmentSkillConfig } from "@services/equipment-skill-config"

type SkillConfigPayload = {
  config?: {
    equipmentSkills?: Record<string, EquipmentSkillConfig>
    skillSlots?: string[]
  }
  rules?: Array<{
    slotKey?: string
    enabled?: boolean
    groups?: unknown
  }>
}

type DraftEquippedItem = {
  instanceId: number
  templateItemId: string
  equipType: "work" | "battle"
  enhanceLevel: number
  durability: number
  maxDurability: number
  state: string
  equippedSlot: string
}

const props = defineProps<{
  token: string
}>()

const { t } = useI18n()
const equipmentStore = useEquipmentStore()
const inventoryStore = useInventoryStore()
const specializationStore = useSpecializationStore()
const { isMobile } = useDevice()

const activeTab = ref<"battle" | "work">("battle")
const loadouts = ref<PlayerLoadoutsPayload>({
  battleLoadouts: [],
  workLoadouts: [],
  workActionBindings: {}
})
const loading = ref(false)
const savingBindings = ref(false)
const savingBattleEditor = ref(false)
const savingWorkEditor = ref(false)
const applyingBattleId = ref("")
const applyingWorkId = ref("")
const deletingBattleId = ref("")
const deletingWorkId = ref("")
const battleDialogVisible = ref(false)
const workDialogVisible = ref(false)
const battleEditorTab = ref<"equipment" | "skills">("equipment")
const battleDialogMode = ref<"create" | "edit">("create")
const workDialogMode = ref<"create" | "edit">("create")

const bindingDraft = reactive<Record<string, string>>({})

const battleEditor = reactive<BattleLoadout>({
  id: "",
  name: "",
  equipment: [],
  skillSlots: [],
  skillRules: []
})

const workEditor = reactive<WorkLoadout>({
  id: "",
  name: "",
  equipment: []
})

const normalizedInventoryItems = computed<InventoryInstance[]>(() =>
  inventoryStore.instances.map((item) => ({
    ...item,
    state: item.state === "equipped" ? "inventory" : item.state
  }))
)

const battleDraftEquippedItems = computed(() => buildDraftEquippedItems(battleEditor.equipment, "battle"))
const workDraftEquippedItems = computed(() => buildDraftEquippedItems(workEditor.equipment, "work"))

const { slotCards: battleDraftSlotCards } = useEquipmentSlots(
  computed(() => "battle" as const),
  {
    equippedItems: battleDraftEquippedItems,
    inventoryItems: normalizedInventoryItems,
    visibleSlots: computed(() => specializationStore.enabledBattleSlotIds)
  }
)

const { slotCards: workDraftSlotCards } = useEquipmentSlots(
  computed(() => "work" as const),
  {
    equippedItems: workDraftEquippedItems,
    inventoryItems: normalizedInventoryItems
  }
)

const actionSections = computed(() =>
  listActionMainTypeDefinitions()
    .filter((section) => {
      const id = String(section.id || "").trim().toLowerCase()
      return id && id !== "battle" && id !== "building"
    })
    .map((section) => ({
      id: String(section.id || "").trim(),
      label: t(section.nameKey)
    }))
)

const battleDialogTitle = computed(() =>
  battleDialogMode.value === "create" ? t("game.loadoutCreateBattle") : t("game.loadoutEditBattle")
)

const workDialogTitle = computed(() =>
  workDialogMode.value === "create" ? t("game.loadoutCreateWork") : t("game.loadoutEditWork")
)
const battleDialogWidth = computed(() => (isMobile.value ? "100%" : "560px"))
const workDialogWidth = computed(() => (isMobile.value ? "100%" : "560px"))

watch(
  () => props.token,
  (token) => {
    if (!token) return
    void loadAll()
  },
  { immediate: true }
)

watch(
  [actionSections, () => loadouts.value.workActionBindings],
  ([sections, bindings]) => {
    const validIDs = new Set(sections.map((section) => section.id))
    for (const key of Object.keys(bindingDraft)) {
      if (!validIDs.has(key)) delete bindingDraft[key]
    }
    for (const section of sections) {
      bindingDraft[section.id] = String(bindings?.[section.id] || "")
    }
  },
  { immediate: true, deep: true }
)

async function loadAll() {
  if (!props.token) return
  loading.value = true
  try {
    await inventoryStore.initialize(props.token)
    loadouts.value = await fetchPlayerLoadouts(props.token)
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.loadoutLoadFailed"))
  } finally {
    loading.value = false
  }
}

function buildDraftEquippedItems(rows: LoadoutEquipmentSlot[], equipType: "work" | "battle") {
  const grouped = new Map<number, string[]>()
  for (const row of rows) {
    const slot = String(row.slot || "").trim()
    const instanceID = Math.trunc(Number(row.instanceId || 0))
    if (!slot || instanceID <= 0) continue
    if (!grouped.has(instanceID)) grouped.set(instanceID, [])
    grouped.get(instanceID)?.push(slot)
  }
  return Array.from(grouped.entries())
    .map(([instanceId, slots]) => {
      const instance = inventoryStore.instances.find((item) => item.instanceId === instanceId)
      if (!instance) return null
      return {
        instanceId,
        templateItemId: instance.templateItemId,
        equipType,
        enhanceLevel: instance.enhanceLevel || 0,
        durability: instance.durability || 0,
        maxDurability: instance.maxDurability || 0,
        state: "equipped",
        equippedSlot: Array.from(new Set(slots)).join(",")
      }
    })
    .filter((row): row is DraftEquippedItem => Boolean(row))
}

function buildGroupedEquipment(items: Array<{ instanceId: number; equippedSlot: string }>) {
  const grouped = new Map<number, Set<string>>()
  for (const item of items) {
    if (!grouped.has(item.instanceId)) grouped.set(item.instanceId, new Set<string>())
    for (const slot of String(item.equippedSlot || "").split(",").map((value) => value.trim()).filter(Boolean)) {
      grouped.get(item.instanceId)?.add(slot)
    }
  }
  return Array.from(grouped.entries())
    .sort((a, b) => a[0] - b[0])
    .flatMap(([instanceId, slots]) =>
      Array.from(slots.values())
        .sort((a, b) => a.localeCompare(b))
        .map((slot) => ({ slot, instanceId }))
    )
}

async function loadCurrentBattleSnapshot(): Promise<BattleLoadout> {
  const skillConfig = await apiGet<SkillConfigPayload>("/api/battle/skills/config", props.token)
  return {
    equipmentSkills: skillConfig.config?.equipmentSkills || {},
    id: "",
    name: t("game.loadoutBattleDefaultName"),
    equipment: buildGroupedEquipment(equipmentStore.equippedItems.filter((item) => item.equipType === "battle")),
    skillSlots: Array.isArray(skillConfig.config?.skillSlots) ? skillConfig.config.skillSlots.map((item) => String(item || "").trim()).filter(Boolean) : [],
    skillRules: Array.isArray(skillConfig.rules)
      ? skillConfig.rules.map((row) => ({
          skillId: String(row.slotKey || "").trim(),
          enabled: Boolean(row.enabled),
          groups: row.groups
        })).filter((row) => row.skillId)
      : []
  }
}

function loadCurrentWorkSnapshot(): WorkLoadout {
  return {
    id: "",
    name: t("game.loadoutWorkDefaultName"),
    equipment: buildGroupedEquipment(equipmentStore.equippedItems.filter((item) => item.equipType === "work"))
  }
}

function hydrateBattleEditor(loadout: BattleLoadout) {
  battleEditor.equipmentSkills = structuredClone(toRaw(loadout.equipmentSkills || {}))
  battleEditor.id = loadout.id
  battleEditor.name = loadout.name
  battleEditor.equipment = loadout.equipment.map((item) => ({ ...item }))
  battleEditor.skillSlots = [...loadout.skillSlots]
  battleEditor.skillRules = loadout.skillRules.map((item) => ({ ...item }))
}

function hydrateWorkEditor(loadout: WorkLoadout) {
  workEditor.id = loadout.id
  workEditor.name = loadout.name
  workEditor.equipment = loadout.equipment.map((item) => ({ ...item }))
}

function createLoadoutID(prefix: "battle" | "work", name: string) {
  const normalized = name.trim().toLowerCase().replace(/[^a-z0-9]+/g, "_").replace(/^_+|_+$/g, "")
  return `${prefix}_${normalized || Date.now()}`
}

async function openCreateBattleDialog() {
  battleDialogMode.value = "create"
  battleEditorTab.value = "equipment"
  hydrateBattleEditor(await loadCurrentBattleSnapshot())
  battleEditor.id = ""
  battleDialogVisible.value = true
}

function openEditBattleDialog(loadout: BattleLoadout) {
  battleDialogMode.value = "edit"
  battleEditorTab.value = "equipment"
  hydrateBattleEditor(loadout)
  battleDialogVisible.value = true
}

function openCreateWorkDialog() {
  workDialogMode.value = "create"
  hydrateWorkEditor(loadCurrentWorkSnapshot())
  workEditor.id = ""
  workDialogVisible.value = true
}

function openEditWorkDialog(loadout: WorkLoadout) {
  workDialogMode.value = "edit"
  hydrateWorkEditor(loadout)
  workDialogVisible.value = true
}

function resolveDraftOccupySlots(instanceId: number, selectedSlot: string) {
  const instance = inventoryStore.instances.find((item) => item.instanceId === instanceId)
  const template = instance ? getEquipmentTemplateDefinition(instance.templateItemId) : undefined
  const occupySlots = Array.isArray(template?.occupySlots)
    ? template?.occupySlots.map((slot) => String(slot || "").trim()).filter(Boolean)
    : []
  if (occupySlots.length > 0) {
    return Array.from(new Set(occupySlots))
  }
  return [selectedSlot]
}

function updateDraftEquipment(rows: LoadoutEquipmentSlot[], payload: { slot: string; instanceId: number }) {
  const slot = String(payload.slot || "").trim()
  const instanceId = Math.max(0, Math.trunc(Number(payload.instanceId || 0)))
  if (!slot || instanceId <= 0) return rows
  const next = rows.filter((row) => row.slot !== slot)
  const occupySlots = resolveDraftOccupySlots(instanceId, slot)
  for (const occupySlot of occupySlots) {
    const normalizedSlot = String(occupySlot || "").trim()
    if (!normalizedSlot) continue
    for (let index = next.length - 1; index >= 0; index -= 1) {
      if (next[index].slot === normalizedSlot) next.splice(index, 1)
    }
    next.push({ slot: normalizedSlot, instanceId })
  }
  return next
    .filter((row) => row.instanceId > 0 && String(row.slot || "").trim())
    .sort((a, b) => a.slot.localeCompare(b.slot) || a.instanceId - b.instanceId)
}

function handleBattleDraftEquip(payload: { slot: string; instanceId: number }) {
  battleEditor.equipment = updateDraftEquipment(battleEditor.equipment, payload)
}

function handleWorkDraftEquip(payload: { slot: string; instanceId: number }) {
  workEditor.equipment = updateDraftEquipment(workEditor.equipment, payload)
}

function handleBattleDraftUnequip(payload: { slot: string; instanceId: number }) {
  battleEditor.equipment = battleEditor.equipment.filter((row) => row.instanceId !== payload.instanceId)
}

function handleWorkDraftUnequip(payload: { slot: string; instanceId: number }) {
  workEditor.equipment = workEditor.equipment.filter((row) => row.instanceId !== payload.instanceId)
}

async function saveBattleEditor() {
  if (!props.token) return
  savingBattleEditor.value = true
  try {
    const name = battleEditor.name.trim() || t("game.loadoutBattleDefaultName")
    const next: BattleLoadout = {
      id: battleEditor.id || createLoadoutID("battle", name),
      name,
      equipment: battleEditor.equipment.map((item) => ({ ...item })),
      skillSlots: [...battleEditor.skillSlots],
      skillRules: battleEditor.skillRules.map((item) => ({ ...item })),
      equipmentSkills: battleEditor.equipmentSkills || {}
    }
    loadouts.value = await upsertBattleLoadout(props.token, next)
    battleDialogVisible.value = false
    ElMessage.success(t("game.saveSuccess"))
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
  } finally {
    savingBattleEditor.value = false
  }
}

async function saveWorkEditor() {
  if (!props.token) return
  savingWorkEditor.value = true
  try {
    const name = workEditor.name.trim() || t("game.loadoutWorkDefaultName")
    const next: WorkLoadout = {
      id: workEditor.id || createLoadoutID("work", name),
      name,
      equipment: workEditor.equipment.map((item) => ({ ...item }))
    }
    loadouts.value = await upsertWorkLoadout(props.token, next)
    workDialogVisible.value = false
    ElMessage.success(t("game.saveSuccess"))
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
  } finally {
    savingWorkEditor.value = false
  }
}

async function applyBattle(loadoutId: string) {
  if (!props.token || !loadoutId) return
  applyingBattleId.value = loadoutId
  try {
    const result = await applyBattleLoadout(props.token, loadoutId)
    await inventoryStore.refreshAll(props.token)
    await loadAll()
    const skippedSkills = Array.isArray(result.skippedSkills) ? result.skippedSkills : []
    if (skippedSkills.length > 0) {
      const skillNames = skippedSkills.map((row) => getLocalizedSkillName(t, row.skillId)).join(" / ")
      ElMessage.warning(t("game.loadoutApplySuccessWithSkippedSkills", { skills: skillNames }))
    } else {
      ElMessage.success(t("game.loadoutApplySuccess"))
    }
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.loadoutApplyFailed"))
  } finally {
    applyingBattleId.value = ""
  }
}

async function applyWork(loadoutId: string) {
  if (!props.token || !loadoutId) return
  applyingWorkId.value = loadoutId
  try {
    await applyWorkLoadout(props.token, loadoutId)
    await inventoryStore.refreshAll(props.token)
    await loadAll()
    ElMessage.success(t("game.loadoutApplySuccess"))
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.loadoutApplyFailed"))
  } finally {
    applyingWorkId.value = ""
  }
}

async function removeLoadout(type: "battle" | "work", loadoutId: string) {
  if (!props.token || !loadoutId) return
  if (type === "battle") deletingBattleId.value = loadoutId
  else deletingWorkId.value = loadoutId
  try {
    loadouts.value = await deletePlayerLoadout(props.token, type, loadoutId)
    ElMessage.success(t("game.loadoutDeleteSuccess"))
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.loadoutDeleteFailed"))
  } finally {
    deletingBattleId.value = ""
    deletingWorkId.value = ""
  }
}

async function saveBindings() {
  if (!props.token) return
  savingBindings.value = true
  try {
    const payload: Record<string, string> = {}
    for (const section of actionSections.value) {
      const value = String(bindingDraft[section.id] || "").trim()
      if (value) payload[section.id] = value
    }
    loadouts.value = await updateWorkLoadoutBindings(props.token, payload)
    ElMessage.success(t("game.saveSuccess"))
  } catch (err) {
    ElMessage.error(resolveErrorMessage(t, err, "game.saveFailed"))
  } finally {
    savingBindings.value = false
  }
}
</script>
