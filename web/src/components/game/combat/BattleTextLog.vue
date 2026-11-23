<template>
  <section class="battle-text-log" aria-labelledby="battle-text-log-title">
    <header class="battle-text-log__header">
      <div class="min-w-0">
        <div id="battle-text-log-title" class="battle-text-log__title">
          <Icon icon="lucide:scroll-text" aria-hidden="true" />
          <span>{{ t("game.battleLogTitle") }}</span>
          <span class="battle-text-log__count">{{ t("game.battleLogCount", { count: visibleLineCount }) }}</span>
        </div>
      </div>
      <div class="battle-text-log__actions">
        <el-tooltip :content="t(followLatest ? 'game.battleLogStopFollowing' : 'game.battleLogFollowLatest')" placement="top">
          <el-button
            circle
            size="small"
            :type="followLatest ? 'primary' : 'default'"
            :aria-label="t(followLatest ? 'game.battleLogStopFollowing' : 'game.battleLogFollowLatest')"
            :aria-pressed="followLatest"
            @click="toggleFollowLatest"
          >
            <Icon icon="lucide:arrow-down-to-line" aria-hidden="true" />
          </el-button>
        </el-tooltip>
        <el-tooltip :content="t('game.battleLogExport')" placement="top">
          <el-button
            circle
            size="small"
            :disabled="!hasLog"
            :aria-label="t('game.battleLogExport')"
            @click="exportLog"
          >
            <Icon icon="lucide:download" aria-hidden="true" />
          </el-button>
        </el-tooltip>
        <el-tooltip :content="t('game.battleLogClear')" placement="top">
          <el-button
            circle
            size="small"
            :disabled="!hasLog"
            :aria-label="t('game.battleLogClear')"
            @click="clearLog"
          >
            <Icon icon="lucide:trash-2" aria-hidden="true" />
          </el-button>
        </el-tooltip>
      </div>
    </header>

    <div class="battle-text-log__filters" role="group" :aria-label="t('game.battleLogFilterLabel')">
      <button
        v-for="option in filterOptions"
        :key="option.value"
        type="button"
        :class="['battle-text-log__filter', { 'battle-text-log__filter--active': activeFilter === option.value }]"
        :aria-pressed="activeFilter === option.value"
        @click="activeFilter = option.value"
      >
        {{ option.label }}
      </button>
    </div>

    <div
      ref="scrollRef"
      class="battle-text-log__viewport"
      role="log"
      tabindex="0"
      aria-live="polite"
      :aria-label="t('game.battleLogTitle')"
      @scroll.passive="handleScroll"
    >
      <div v-if="visibleGroups.length === 0" class="battle-text-log__empty">
        <Icon icon="lucide:scroll" aria-hidden="true" />
        <span>{{ t(!hasLog ? "game.battleLogEmpty" : "game.battleLogFilterEmpty") }}</span>
      </div>
      <div v-else class="relative w-full" :style="{ height: `${rowVirtualizer.getTotalSize()}px` }">
        <article
          v-for="entry in virtualGroups"
          :key="String(entry.virtual.key)"
          :ref="(element) => measureGroup(element)"
          :data-index="entry.virtual.index"
          class="battle-text-log__group"
          :style="{ transform: `translateY(${entry.virtual.start}px)` }"
        >
          <div v-if="entry.group.eventType !== COMPRESSED_EVENT_TYPE" class="battle-text-log__group-meta">
            <span v-if="entry.group.ts > 0">{{ timestampLabel(entry.group.ts) }}</span>
            <span>{{ t("game.battleLogWave", { wave: Math.max(1, entry.group.wave) }) }}</span>
            <span>#{{ entry.group.seq }}</span>
            <BattleLogEntityRef v-if="entry.group.actor" :reference="entry.group.actor" />
            <span v-if="entry.group.preparationTurn" class="battle-text-log__flag battle-text-log__flag--preparation">
              {{ t("game.battleLogPreparationTurn") }}
            </span>
            <span v-if="entry.group.extraTurn" class="battle-text-log__flag battle-text-log__flag--extra">
              {{ t("game.battleLogExtraTurnFlag") }}
            </span>
          </div>

          <div class="battle-text-log__lines">
            <div
              v-for="line in entry.group.lines"
              :key="line.id"
              :class="['battle-text-log__line', `battle-text-log__line--${line.category}`]"
            >
              <span class="battle-text-log__line-icon" aria-hidden="true">
                <Icon :icon="line.icon" />
              </span>
              <div class="battle-text-log__line-body">
                <div class="battle-text-log__message">
                  <span v-if="line.phase" class="battle-text-log__phase">{{ phaseLabel(line.phase) }}</span>
                  <I18nT :keypath="line.messageKey" tag="span" scope="global">
                    <template v-if="line.refs.source" #source>
                      <BattleLogEntityRef :reference="line.refs.source" />
                    </template>
                    <template v-if="line.refs.target" #target>
                      <BattleLogEntityRef :reference="line.refs.target" />
                    </template>
                    <template v-if="line.refs.unit" #unit>
                      <BattleLogEntityRef :reference="line.refs.unit" />
                    </template>
                    <template v-if="line.refs.skill" #skill>
                      <BattleLogEntityRef :reference="line.refs.skill" />
                    </template>
                    <template v-if="line.refs.item" #item>
                      <BattleLogEntityRef :reference="line.refs.item" />
                    </template>
                    <template v-if="line.refs.buff" #buff>
                      <BattleLogEntityRef :reference="line.refs.buff" />
                    </template>
                    <template v-if="line.refs.summon" #summon>
                      <BattleLogEntityRef :reference="line.refs.summon" />
                    </template>
                    <template v-if="line.values.wave !== undefined" #wave>{{ line.values.wave }}</template>
                    <template v-if="line.values.players !== undefined" #players>{{ line.values.players }}</template>
                    <template v-if="line.values.enemies !== undefined" #enemies>{{ line.values.enemies }}</template>
                    <template v-if="line.values.position !== undefined" #position>{{ line.values.position }}</template>
                    <template v-if="line.values.completed !== undefined" #completed>{{ line.values.completed }}</template>
                    <template v-if="line.values.reason !== undefined" #reason>{{ line.values.reason }}</template>
                    <template v-if="line.values.event !== undefined" #event>{{ line.values.event }}</template>
                    <template v-if="line.values.field !== undefined" #field>{{ line.values.field }}</template>
                    <template v-if="line.values.remaining !== undefined" #remaining>{{ line.values.remaining }}</template>
                    <template v-if="line.values.progress !== undefined" #progress>{{ line.values.progress }}</template>
                    <template v-if="line.values.goal !== undefined" #goal>{{ line.values.goal }}</template>
                    <template v-if="line.values.outcome !== undefined" #outcome>{{ line.values.outcome }}</template>
                    <template v-if="line.values.amount !== undefined" #amount>{{ line.values.amount }}</template>
                    <template v-if="line.values.window !== undefined" #window>{{ line.values.window }}</template>
                    <template v-if="line.values.damageType !== undefined" #damageType>{{ line.values.damageType }}</template>
                    <template v-if="line.values.critical !== undefined" #critical>{{ line.values.critical }}</template>
                    <template v-if="line.values.resource !== undefined" #resource>{{ line.values.resource }}</template>
                    <template v-if="line.values.from !== undefined" #from>{{ line.values.from }}</template>
                    <template v-if="line.values.to !== undefined" #to>{{ line.values.to }}</template>
                    <template v-if="line.values.effect !== undefined" #effect>{{ line.values.effect }}</template>
                    <template v-if="line.values.count !== undefined" #count>{{ line.values.count }}</template>
                    <template v-if="line.values.fromWave !== undefined" #fromWave>{{ line.values.fromWave }}</template>
                    <template v-if="line.values.toWave !== undefined" #toWave>{{ line.values.toWave }}</template>
                  </I18nT>
                </div>
                <div v-if="line.details.length > 0" class="battle-text-log__details">
                  <span
                    v-for="(detail, detailIndex) in line.details"
                    :key="`${line.id}-detail-${detailIndex}`"
                    :class="['battle-text-log__detail', `battle-text-log__detail--${detail.tone || 'default'}`]"
                  >
                    {{ detail.text }}
                  </span>
                </div>
                <div v-if="line.relatedRefs.length > 0" class="battle-text-log__related">
                  <span>{{ t("game.battleLogRelated") }}</span>
                  <BattleLogEntityRef
                    v-for="reference in line.relatedRefs"
                    :key="`${reference.kind}:${reference.id}`"
                    :reference="reference"
                  />
                </div>
              </div>
            </div>
          </div>
        </article>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch, type ComponentPublicInstance } from "vue"
import { Translation as I18nT, useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import { ElMessage } from "element-plus"
import { useVirtualizer, type VirtualItem } from "@tanstack/vue-virtual"
import BattleLogEntityRef from "@components/game/combat/BattleLogEntityRef.vue"
import { getAttributeDefinition, formatAttributeValueByKey } from "@services/attribute-display"
import { getLocalizedBuffName } from "@services/buff-definition"
import { buildBattleSpecializationSourceLabel } from "@services/battle-specialization-source-display"
import { getEnemyDefinition, getLocalizedEnemyName } from "@services/enemy-definition"
import { getLocalizedItemName } from "@services/item-label"
import { getLocalizedSkillName } from "@services/skill-definition"
import {
  battleLogEventKey,
  buildBattleLogGroup,
  compressBattleLogGroups,
  type BattleLogCategory,
  type BattleLogEntityReference,
  type BattleLogEvent,
  type BattleLogGroup
} from "@services/battle-log"

type BattleLogUnit = Record<string, unknown> & {
  unitId?: string | number
  enemyId?: string
  name?: string
  nameKey?: string
}

type FilterValue = "all" | Exclude<BattleLogCategory, "system">
type VisibleGroup = BattleLogGroup & { lines: BattleLogGroup["lines"] }

const MAX_DETAILED_LOG_LINES = 1000
const COMPRESS_BATCH_LINES = 50
const MAX_PROCESSED_KEYS = 10000
const MAX_UNIT_REGISTRY_ENTRIES = 2000
const COMPRESSED_EVENT_TYPE = "battle.log.compressed"

const props = defineProps<{
  roomId: number
  runId: number
  events: BattleLogEvent[]
  units: BattleLogUnit[]
  active?: boolean
}>()

const { t, locale } = useI18n()
const groups = ref<BattleLogGroup[]>([])
const compressedLineCount = ref(0)
const compressedFirstWave = ref(0)
const compressedLastWave = ref(0)
const activeFilter = ref<FilterValue>("all")
const followLatest = ref(true)
const scrollRef = ref<HTMLElement | null>(null)
const unitRegistry = new Map<string, BattleLogUnit>()
const processedKeys = new Set<string>()
const processedKeyOrder: string[] = []
let activeRoomId = 0
let followFrame = 0

const filterOptions = computed<Array<{ value: FilterValue; label: string }>>(() => [
  { value: "all", label: t("game.battleLogFilterAll") },
  { value: "action", label: t("game.battleLogFilterAction") },
  { value: "damage", label: t("game.battleLogFilterDamage") },
  { value: "support", label: t("game.battleLogFilterSupport") },
  { value: "status", label: t("game.battleLogFilterStatus") },
  { value: "field", label: t("game.battleLogFilterField") }
])

const compressedGroup = computed<BattleLogGroup | null>(() => {
  if (compressedLineCount.value <= 0) return null
  return {
    id: `battle-log-compressed:${activeRoomId}`,
    runId: 0,
    seq: 0,
    ts: 0,
    wave: compressedFirstWave.value || 1,
    eventType: COMPRESSED_EVENT_TYPE,
    preparationTurn: false,
    extraTurn: false,
    turnKind: "",
    lines: [{
      id: `battle-log-compressed:${activeRoomId}:line`,
      kind: "compressed",
      category: "system",
      icon: "lucide:archive",
      messageKey: "game.battleLogCompressed",
      values: {
        count: compressedLineCount.value,
        fromWave: compressedFirstWave.value || 1,
        toWave: compressedLastWave.value || compressedFirstWave.value || 1
      },
      refs: {},
      details: [],
      relatedRefs: []
    }]
  }
})

const storedGroups = computed<BattleLogGroup[]>(() => compressedGroup.value
  ? [compressedGroup.value, ...groups.value]
  : groups.value)

const hasLog = computed(() => storedGroups.value.length > 0)

const visibleGroups = computed<VisibleGroup[]>(() => storedGroups.value.flatMap((group) => {
  if (activeFilter.value === "all") return [group]
  const lines = group.lines.filter((line) => line.category === activeFilter.value)
  return lines.length > 0 ? [{ ...group, lines }] : []
}))

const visibleLineCount = computed(() => visibleGroups.value.reduce((total, group) => total + group.lines.length, 0))

const rowVirtualizer = useVirtualizer<HTMLElement, HTMLElement>(computed(() => ({
  count: visibleGroups.value.length,
  getScrollElement: () => scrollRef.value,
  estimateSize: (index: number) => 46 + (visibleGroups.value[index]?.lines.length || 1) * 46,
  getItemKey: (index: number) => visibleGroups.value[index]?.id || index,
  overscan: 5
})))

const virtualGroups = computed(() => rowVirtualizer.value.getVirtualItems().flatMap((virtual: VirtualItem) => {
  const group = visibleGroups.value[virtual.index]
  return group ? [{ virtual, group }] : []
}))

watch(
  () => ({
    roomId: Math.max(0, Math.trunc(Number(props.roomId) || 0)),
    runId: Math.max(0, Math.trunc(Number(props.runId) || 0)),
    events: props.events,
    units: props.units
  }),
  (snapshot) => {
    if (activeRoomId !== snapshot.roomId) {
      resetForRoom(snapshot.roomId)
    }
    if (snapshot.roomId <= 0) return
    registerUnits(snapshot.units)
    for (const event of snapshot.events) {
      registerEventUnits(event)
      const key = battleLogEventKey(snapshot.runId, event)
      if (processedKeys.has(key)) continue
      rememberProcessedKey(key)
      const group = buildBattleLogGroup(event, {
        runId: snapshot.runId,
        t,
        resolveUnit,
        positionLabel,
        sideLabel,
        damageTypeLabel,
        resourceLabel,
        resolveFieldEffectName,
        resolveSourceLabel: (meta) => buildBattleSpecializationSourceLabel(meta, t),
        formatStatModifier
      })
      if (group) groups.value.push(group)
    }
    trimHistory()
  },
  { deep: true, immediate: true }
)

watch(
  () => `${visibleGroups.value.length}:${visibleLineCount.value}:${activeFilter.value}`,
  async () => {
    if (props.active === false || !followLatest.value || visibleGroups.value.length === 0) return
    await nextTick()
    scrollToLatest()
  },
  { immediate: true }
)

watch(locale, async () => {
  rowVirtualizer.value.measure()
  await nextTick()
  if (props.active !== false && followLatest.value) scrollToLatest()
})

watch(
  () => props.active,
  async (active) => {
    if (active === false) return
    await nextTick()
    rowVirtualizer.value.measure()
    await nextTick()
    if (followLatest.value) scrollToLatest()
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  if (followFrame) window.cancelAnimationFrame(followFrame)
})

function resetForRoom(roomId: number) {
  activeRoomId = roomId
  groups.value = []
  resetCompression()
  unitRegistry.clear()
  processedKeys.clear()
  processedKeyOrder.splice(0)
  followLatest.value = true
}

function registerUnits(units: BattleLogUnit[]) {
  for (const unit of Array.isArray(units) ? units : []) {
    const unitId = normalizeId(unit.unitId)
    if (!unitId) continue
    const existing = unitRegistry.get(unitId) || {}
    unitRegistry.delete(unitId)
    unitRegistry.set(unitId, { ...existing, ...unit, unitId })
    while (unitRegistry.size > MAX_UNIT_REGISTRY_ENTRIES) {
      const oldest = unitRegistry.keys().next().value
      if (typeof oldest !== "string") break
      unitRegistry.delete(oldest)
    }
  }
}

function registerEventUnits(event: BattleLogEvent) {
  const data = asRecord(event.data)
  registerUnits(recordArray(data.players))
  registerUnits(recordArray(data.enemies))
  registerUnits(recordArray(data.unitUpdates))
  for (const step of recordArray(data.steps ?? data.p)) {
    const summon = asRecord(step.summon ?? step.q)
    if (Object.keys(summon).length > 0) {
      registerUnits([{
        ...summon,
        unitId: normalizeId(summon.unitId ?? summon.u),
        name: String(summon.name ?? summon.n ?? "").trim(),
        nameKey: String(summon.nameKey ?? summon.k ?? "").trim(),
        side: summon.side ?? summon.s,
        position: summon.position ?? summon.p
      }])
    }
  }
}

function resolveUnit(unitId: string, snapshot?: Record<string, unknown>): BattleLogEntityReference {
  const normalized = normalizeId(unitId)
  if (snapshot && Object.keys(snapshot).length > 0) {
    registerUnits([{ ...(unitRegistry.get(normalized) || {}), ...snapshot, unitId: normalized }])
  }
  const unit = unitRegistry.get(normalized) || {}
  const enemyId = inferEnemyId(normalized, unit)
  return {
    kind: "unit",
    id: normalized,
    label: resolveUnitLabel(normalized, unit, enemyId),
    enemyId
  }
}

function resolveUnitLabel(unitId: string, unit: BattleLogUnit, enemyId?: string): string {
  const nameKey = String(unit.nameKey || "").trim()
  if (nameKey) {
    const localized = t(nameKey)
    if (localized && localized !== nameKey) return localized
  }
  const name = String(unit.name || "").trim()
  if (name) return name
  if (enemyId) return getLocalizedEnemyName(t, enemyId, enemyId)
  return unitId || t("game.battleLogUnknownUnit")
}

function inferEnemyId(unitId: string, unit: BattleLogUnit): string | undefined {
  const direct = normalizeId(unit.enemyId ?? unit.summonTemplateUnitId)
  if (direct && getEnemyDefinition(direct)) return direct
  if (getEnemyDefinition(unitId)) return unitId
  if (!unitId.toLowerCase().startsWith("enemy:")) return undefined
  const candidate = unitId.split(":").at(-1) || ""
  return getEnemyDefinition(candidate) ? candidate : undefined
}

function positionLabel(position: string): string {
  switch (String(position || "").trim().toLowerCase()) {
    case "vanguard":
      return t("game.battlePositionVanguard")
    case "midguard":
      return t("game.battlePositionMidguard")
    case "rearguard":
      return t("game.battlePositionRearguard")
    default:
      return String(position || "").trim() || t("game.battleLogUnknownPosition")
  }
}

function sideLabel(side: string): string {
  const normalized = String(side || "").trim().toLowerCase()
  if (normalized === "player" || normalized === "ally" || normalized === "p") return t("game.battlePlayerSide")
  if (normalized === "enemy" || normalized === "e") return t("game.battleEnemySide")
  return normalized || t("game.battleLogUnknownSide")
}

function damageTypeLabel(damageType: string): string {
  const normalized = String(damageType || "").trim().toLowerCase()
  if (normalized === "physical") return t("game.battleLogDamageTypePhysical")
  if (normalized === "magical" || normalized === "magic") return t("game.battleLogDamageTypeMagical")
  return t("game.battleLogDamageTypeGeneric")
}

function resourceLabel(resource: string): string {
  return String(resource || "").trim().toLowerCase() === "hp"
    ? t("game.battleLogResourceHp")
    : t("game.battleLogResourceMp")
}

function resolveFieldEffectName(effectId: string): string {
  const normalized = normalizeId(effectId)
  if (!normalized) return t("game.battleLogUnknownField")
  const key = `battlefield_effect.${normalized}.name`
  const localized = t(key)
  return localized && localized !== key ? localized : normalized
}

function formatStatModifier(key: string, value: number): string {
  const definition = getAttributeDefinition(key)
  const nameKey = String(definition?.nameKey || "").trim()
  const localized = nameKey ? t(nameKey) : ""
  const label = localized && localized !== nameKey ? localized : key
  return `${label} ${formatAttributeValueByKey(key, value, true)}`
}

function phaseLabel(phase: string): string {
  if (phase === "item") return t("game.battleLogPhaseItem")
  if (phase === "regen") return t("game.battleLogPhaseRegen")
  return t("game.battleLogPhaseAction")
}

function timestampLabel(raw: number): string {
  const numeric = Number(raw)
  if (!Number.isFinite(numeric) || numeric <= 0) return ""
  const timestamp = numeric < 1_000_000_000_000 ? numeric * 1000 : numeric
  const date = new Date(timestamp)
  return Number.isNaN(date.getTime()) ? "" : date.toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit"
  })
}

function measureGroup(element: Element | ComponentPublicInstance | null) {
  const node = element instanceof Element ? element : element?.$el
  if (node instanceof HTMLElement) rowVirtualizer.value.measureElement(node)
}

function handleScroll() {
  const element = scrollRef.value
  if (!element) return
  const distanceToBottom = element.scrollHeight - element.clientHeight - element.scrollTop
  if (distanceToBottom > 72) followLatest.value = false
  else if (distanceToBottom < 12) followLatest.value = true
}

function toggleFollowLatest() {
  followLatest.value = !followLatest.value
  if (followLatest.value && props.active !== false) nextTick(scrollToLatest)
}

function scrollToLatest() {
  if (props.active === false) return
  const lastIndex = visibleGroups.value.length - 1
  if (lastIndex < 0) return
  rowVirtualizer.value.scrollToIndex(lastIndex, { align: "end" })
  if (followFrame) window.cancelAnimationFrame(followFrame)
  followFrame = window.requestAnimationFrame(() => {
    followFrame = window.requestAnimationFrame(() => {
      followFrame = 0
      if (props.active === false || !followLatest.value || !scrollRef.value) return
      scrollRef.value.scrollTop = scrollRef.value.scrollHeight
    })
  })
}

function clearLog() {
  groups.value = []
  resetCompression()
}

function rememberProcessedKey(key: string) {
  processedKeys.add(key)
  processedKeyOrder.push(key)
  while (processedKeyOrder.length > MAX_PROCESSED_KEYS) {
    const stale = processedKeyOrder.shift()
    if (stale) processedKeys.delete(stale)
  }
}

function trimHistory() {
  const compressed = compressBattleLogGroups(groups.value, MAX_DETAILED_LOG_LINES, COMPRESS_BATCH_LINES)
  if (compressed.removedLineCount <= 0) return
  groups.value = compressed.groups
  compressedLineCount.value += compressed.removedLineCount
  if (compressedFirstWave.value === 0) compressedFirstWave.value = compressed.firstWave
  if (compressed.lastWave > 0) compressedLastWave.value = compressed.lastWave
}

function resetCompression() {
  compressedLineCount.value = 0
  compressedFirstWave.value = 0
  compressedLastWave.value = 0
}

function exportLog() {
  if (!hasLog.value) return
  try {
    const text = storedGroups.value.flatMap((group) => {
      const prefix = group.eventType === COMPRESSED_EVENT_TYPE
        ? ""
        : [
            timestampLabel(group.ts),
            t("game.battleLogWave", { wave: Math.max(1, group.wave) }),
            `#${group.seq}`,
            group.actor ? entityText(group.actor) : ""
          ].filter(Boolean).join(" · ")
      return group.lines.map((line) => {
        const values: Record<string, string | number> = { ...line.values }
        for (const [name, reference] of Object.entries(line.refs)) values[name] = entityText(reference)
        const message = t(line.messageKey, values)
        const details = line.details.length > 0 ? ` [${line.details.map((detail) => detail.text).join("; ")}]` : ""
        const related = line.relatedRefs.length > 0
          ? ` | ${t("game.battleLogRelated")} ${line.relatedRefs.map(entityText).join(", ")}`
          : ""
        const phase = line.phase ? ` ${phaseLabel(line.phase)}` : ""
        return `${prefix}${phase} ${message}${details}${related}`.trim()
      })
    }).join("\n")
    const blob = new Blob([text], { type: "text/plain;charset=utf-8" })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement("a")
    anchor.href = url
    anchor.download = `battle-log-room-${Math.max(0, Math.trunc(Number(props.roomId) || 0))}.txt`
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
    URL.revokeObjectURL(url)
    ElMessage.success(t("game.battleLogExported"))
  } catch {
    ElMessage.error(t("game.battleLogExportFailed"))
  }
}

function entityText(reference: BattleLogEntityReference): string {
  if (reference.label) return reference.label
  if (reference.kind === "skill") return getLocalizedSkillName(t, reference.id, reference.id)
  if (reference.kind === "item") return getLocalizedItemName(t, reference.id, { fallbackText: reference.id })
  if (reference.kind === "buff") return getLocalizedBuffName(t, reference.id, reference.id)
  if (reference.enemyId) return getLocalizedEnemyName(t, reference.enemyId, reference.id)
  return reference.id
}

function normalizeId(raw: unknown): string {
  const value = String(raw || "").trim()
  return value === "<nil>" || value === "nil" || value === "null" ? "" : value
}

function asRecord(raw: unknown): Record<string, unknown> {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {}
  return raw as Record<string, unknown>
}

function recordArray(raw: unknown): BattleLogUnit[] {
  if (!Array.isArray(raw)) return []
  return raw.flatMap((entry) => {
    const row = asRecord(entry)
    return Object.keys(row).length > 0 ? [row as BattleLogUnit] : []
  })
}
</script>

<style scoped>
.battle-text-log {
  overflow: hidden;
  border: 1px solid var(--app-border);
  border-radius: 6px;
  background: color-mix(in srgb, var(--app-surface) 96%, var(--el-fill-color-light) 4%);
}

.battle-text-log__header {
  display: flex;
  min-height: 46px;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--app-border);
}

.battle-text-log__title,
.battle-text-log__actions,
.battle-text-log__group-meta,
.battle-text-log__message,
.battle-text-log__details,
.battle-text-log__related {
  display: flex;
  align-items: center;
}

.battle-text-log__title {
  min-width: 0;
  gap: 7px;
  color: var(--el-text-color-primary);
  font-size: 14px;
  font-weight: 700;
}

.battle-text-log__count {
  color: var(--app-muted);
  font-size: 11px;
  font-weight: 500;
}

.battle-text-log__actions {
  flex: none;
  gap: 6px;
}

.battle-text-log__actions :deep(.el-button + .el-button) {
  margin-left: 0;
}

.battle-text-log__filters {
  display: flex;
  gap: 0;
  overflow-x: auto;
  padding: 8px 10px;
  border-bottom: 1px solid var(--app-border);
  scrollbar-width: thin;
}

.battle-text-log__filter {
  flex: none;
  min-height: 30px;
  padding: 5px 11px;
  border: 1px solid var(--app-border);
  border-right-width: 0;
  color: var(--app-muted);
  background: var(--app-surface);
  font: inherit;
  font-size: 12px;
  line-height: 1.2;
  cursor: pointer;
}

.battle-text-log__filter:first-child {
  border-radius: 5px 0 0 5px;
}

.battle-text-log__filter:last-child {
  border-right-width: 1px;
  border-radius: 0 5px 5px 0;
}

.battle-text-log__filter:hover {
  color: var(--el-text-color-primary);
  background: var(--el-fill-color-light);
}

.battle-text-log__filter--active {
  z-index: 1;
  border-color: var(--el-color-primary);
  color: var(--el-color-primary);
  background: color-mix(in srgb, var(--el-color-primary) 8%, var(--app-surface));
  font-weight: 650;
}

.battle-text-log__filter--active + .battle-text-log__filter {
  border-left-color: var(--el-color-primary);
}

.battle-text-log__viewport {
  position: relative;
  height: 420px;
  overflow-y: auto;
  overscroll-behavior: contain;
}

.battle-text-log__empty {
  display: grid;
  min-height: 220px;
  place-content: center;
  justify-items: center;
  gap: 8px;
  padding: 24px;
  color: var(--app-muted);
  font-size: 12px;
  text-align: center;
}

.battle-text-log__empty svg {
  width: 24px;
  height: 24px;
  opacity: 0.55;
}

.battle-text-log__group {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  padding: 8px 10px 9px;
  border-bottom: 1px solid color-mix(in srgb, var(--app-border) 70%, transparent);
}

.battle-text-log__group-meta {
  min-height: 20px;
  flex-wrap: wrap;
  gap: 5px 8px;
  margin-bottom: 4px;
  color: var(--app-muted);
  font-size: 10px;
  font-variant-numeric: tabular-nums;
}

.battle-text-log__flag,
.battle-text-log__phase,
.battle-text-log__detail {
  display: inline-flex;
  align-items: center;
  border-radius: 4px;
  white-space: nowrap;
}

.battle-text-log__flag {
  padding: 2px 5px;
  font-weight: 650;
}

.battle-text-log__flag--preparation {
  color: #0f766e;
  background: color-mix(in srgb, #14b8a6 12%, transparent);
}

.battle-text-log__flag--extra {
  color: #7c3aed;
  background: color-mix(in srgb, #8b5cf6 12%, transparent);
}

.battle-text-log__lines {
  display: grid;
  gap: 2px;
}

.battle-text-log__line {
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr);
  gap: 7px;
  min-height: 36px;
  padding: 4px 0;
}

.battle-text-log__line-icon {
  display: grid;
  width: 26px;
  height: 26px;
  place-items: center;
  margin-top: 1px;
  border: 1px solid color-mix(in srgb, currentColor 26%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, currentColor 8%, transparent);
  color: #475569;
}

.battle-text-log__line--action .battle-text-log__line-icon { color: #7c3aed; }
.battle-text-log__line--damage .battle-text-log__line-icon { color: #dc2626; }
.battle-text-log__line--support .battle-text-log__line-icon { color: #15803d; }
.battle-text-log__line--status .battle-text-log__line-icon { color: #b45309; }
.battle-text-log__line--field .battle-text-log__line-icon { color: #0369a1; }
.battle-text-log__line--system .battle-text-log__line-icon { color: #475569; }

.battle-text-log__line-body {
  min-width: 0;
  padding-top: 1px;
}

.battle-text-log__message {
  min-width: 0;
  flex-wrap: wrap;
  gap: 4px;
  color: var(--el-text-color-primary);
  font-size: 12px;
  line-height: 1.65;
  overflow-wrap: anywhere;
}

.battle-text-log__phase {
  flex: none;
  padding: 1px 5px;
  color: var(--app-muted);
  background: var(--el-fill-color-light);
  font-size: 10px;
  font-weight: 650;
}

.battle-text-log__details,
.battle-text-log__related {
  min-width: 0;
  flex-wrap: wrap;
  gap: 4px 6px;
  margin-top: 3px;
  color: var(--app-muted);
  font-size: 10px;
  line-height: 1.5;
}

.battle-text-log__detail {
  padding: 1px 5px;
  background: var(--el-fill-color-light);
}

.battle-text-log__detail--danger { color: #dc2626; }
.battle-text-log__detail--success { color: #15803d; }
.battle-text-log__detail--warning { color: #b45309; }
.battle-text-log__detail--info { color: #0369a1; }

.battle-text-log__related {
  padding-top: 2px;
}

@media (max-width: 767px) {
  .battle-text-log__header {
    padding: 8px;
  }

  .battle-text-log__filters {
    padding: 7px 8px;
  }

  .battle-text-log__filter {
    padding-right: 9px;
    padding-left: 9px;
  }

  .battle-text-log__viewport {
    height: min(380px, 52dvh);
    min-height: 300px;
  }

  .battle-text-log__group {
    padding-right: 8px;
    padding-left: 8px;
  }
}

:global(.dark) .battle-text-log__flag--preparation { color: #5eead4; }
:global(.dark) .battle-text-log__flag--extra { color: #c4b5fd; }
:global(.dark) .battle-text-log__line--action .battle-text-log__line-icon { color: #c4b5fd; }
:global(.dark) .battle-text-log__line--damage .battle-text-log__line-icon { color: #fda4af; }
:global(.dark) .battle-text-log__line--support .battle-text-log__line-icon { color: #86efac; }
:global(.dark) .battle-text-log__line--status .battle-text-log__line-icon { color: #fcd34d; }
:global(.dark) .battle-text-log__line--field .battle-text-log__line-icon { color: #7dd3fc; }
:global(.dark) .battle-text-log__line--system .battle-text-log__line-icon { color: #cbd5e1; }
:global(.dark) .battle-text-log__detail--danger { color: #fda4af; }
:global(.dark) .battle-text-log__detail--success { color: #86efac; }
:global(.dark) .battle-text-log__detail--warning { color: #fcd34d; }
:global(.dark) .battle-text-log__detail--info { color: #7dd3fc; }
</style>
