<template>
  <div v-if="groups.length || details.length" class="space-y-2">
    <template v-if="visibleGroups.length">
      <div
        v-for="group in visibleGroups"
        :key="`basic-attack-group-${group.key}`"
        class="basic-attack-effect-block"
        :class="compact ? 'basic-attack-effect-block--compact' : ''"
      >
        <div class="basic-attack-label">{{ group.title }}</div>
        <div class="grid grid-cols-2 gap-x-4 gap-y-2">
          <div v-for="detail in getGroupDetails(group)" :key="`basic-attack-line-${group.key}-${detail.key}`" class="basic-attack-row">
            <span class="basic-attack-label">
              {{ detail.label }}
              <el-tooltip v-if="detail.labelHint" :content="detail.labelHint" placement="top">
                <el-icon class="basic-attack-label-hint"><QuestionFilled /></el-icon>
              </el-tooltip>
            </span>
            <div v-if="Array.isArray(detail.buffIds) && detail.buffIds.length > 0" class="basic-attack-buff-list">
              <BuffQuickRef
                v-for="buffId in detail.buffIds"
                :key="`basic-attack-buff-${group.key}-${detail.key}-${buffId}`"
                :buff-id="buffId"
                :payload-formula="detail.buffPayloadFormula"
              />
            </div>
            <span v-else class="basic-attack-value">{{ detail.value }}</span>
          </div>
        </div>
      </div>
    </template>

    <div v-if="visibleDetails.length" class="grid grid-cols-2 gap-x-4 gap-y-2">
      <div v-for="detail in visibleDetails" :key="`basic-attack-detail-${detail.key}`" class="basic-attack-row">
        <span class="basic-attack-label">
          {{ detail.label }}
          <el-tooltip v-if="detail.labelHint" :content="detail.labelHint" placement="top">
            <el-icon class="basic-attack-label-hint"><QuestionFilled /></el-icon>
          </el-tooltip>
        </span>
        <div v-if="Array.isArray(detail.buffIds) && detail.buffIds.length > 0" class="basic-attack-buff-list">
          <BuffQuickRef
            v-for="buffId in detail.buffIds"
            :key="`basic-attack-detail-buff-${detail.key}-${buffId}`"
            :buff-id="buffId"
            :payload-formula="detail.buffPayloadFormula"
          />
        </div>
        <span v-else class="basic-attack-value">{{ detail.value }}</span>
      </div>
    </div>

    <div v-if="showToggle" class="basic-attack-toggle">
      <el-button link type="primary" class="basic-attack-toggle-btn" @click="expanded = !expanded">
        <!-- {{ expanded ? t("game.basicAttackDetailCollapse") : t("game.basicAttackDetailExpand") }} -->
        <span class="basic-attack-toggle-count">{{ toggleCountLabel }}</span>
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { useI18n } from "vue-i18n"
import { QuestionFilled } from "@element-plus/icons-vue"
import BuffQuickRef from "@components/game/buff/BuffQuickRef.vue"

type BasicAttackDetail = {
  key: string
  label: string
  value: string
  buffIds?: string[]
  buffPayloadFormula?: Record<string, unknown>
  labelHint?: string
}

type BasicAttackGroup = {
  key: string
  title: string
  details: BasicAttackDetail[]
  linkage?: {
    onlyWhenNoTargetFromEffects?: number[]
    reuseTargetsFromEffect?: number
    mpCostGatedRefs?: number[]
  }
}

const props = withDefaults(
  defineProps<{
    groups?: BasicAttackGroup[]
    details?: BasicAttackDetail[]
    compact?: boolean
    maxGroups?: number
    maxRowsPerGroup?: number
    initialExpanded?: boolean
  }>(),
  {
    groups: () => [],
    details: () => [],
    compact: false,
    maxGroups: 1,
    maxRowsPerGroup: 4,
    initialExpanded: false
  }
)

const { t } = useI18n()
const expanded = ref(props.initialExpanded)

const visibleGroups = computed(() => {
  if (!props.compact || expanded.value) return props.groups
  return props.groups.slice(0, Math.max(1, Math.trunc(Number(props.maxGroups || 1))))
})

const visibleDetails = computed(() => {
  if (!props.compact || expanded.value) return props.details
  return props.details.slice(0, Math.max(1, Math.trunc(Number(props.maxRowsPerGroup || 4))))
})

const hiddenGroupCount = computed(() => Math.max(0, props.groups.length - visibleGroups.value.length))
const hiddenDetailCount = computed(() => Math.max(0, props.details.length - visibleDetails.value.length))
const hiddenPerGroupCount = computed(() => {
  if (!props.compact || expanded.value) return 0
  const maxRows = Math.max(1, Math.trunc(Number(props.maxRowsPerGroup || 4)))
  return visibleGroups.value.reduce((sum, group) => sum + Math.max(0, getGroupDetails(group).length - maxRows), 0)
})
const hiddenCount = computed(() => hiddenGroupCount.value + hiddenDetailCount.value + hiddenPerGroupCount.value)
const showToggle = computed(() => props.compact && hiddenCount.value > 0)
const toggleCountLabel = computed(() => (hiddenCount.value > 0 ? t("game.basicAttackDetailHiddenCount", { count: hiddenCount.value }) : ""))

function formatEffectRefs(effectRefs: number[]): string {
  return effectRefs.map((index) => t("game.effectRefIndex", { index })).join(", ")
}

function getGroupDetails(group: BasicAttackGroup): BasicAttackDetail[] {
  let out = [...group.details]
  const linkage = group.linkage
  if (!linkage) return out
  const reuseTargetsFromEffect = Number(linkage.reuseTargetsFromEffect)
  if (Number.isFinite(reuseTargetsFromEffect) && reuseTargetsFromEffect > 0) {
    const inheritedTargetingKeys = new Set(["target-direction", "seek-strategy", "target-seek", "target-mode"])
    out = out.filter((detail) => !inheritedTargetingKeys.has(detail.key))
    out.push({
      key: "reuse-targets-from-effect",
      label: t("game.effectReuseTargetsFromEffect"),
      value: formatEffectRefs([Math.trunc(reuseTargetsFromEffect)])
    })
  }
  if (Array.isArray(linkage.onlyWhenNoTargetFromEffects) && linkage.onlyWhenNoTargetFromEffects.length > 0) {
    const refs = linkage.onlyWhenNoTargetFromEffects
      .map((value) => Math.trunc(Number(value)))
      .filter((value) => Number.isFinite(value) && value > 0)
    const mpGatedRefs = Array.isArray(linkage.mpCostGatedRefs) ? linkage.mpCostGatedRefs : []
    const hasMpGated = refs.some((ref) => mpGatedRefs.includes(ref))
    const labelKey = hasMpGated ? "game.effectOnlyWhenNoTargetOrMpInsufficient" : "game.effectOnlyWhenNoTargetFromEffects"
    const labelFallback = hasMpGated ? "备选触发（无目标 或 魔法不足）" : t("game.effectOnlyWhenNoTargetFromEffects")
    const label = t(labelKey)
    out.push({
      key: "only-when-no-target-from-effects",
      label: label === labelKey ? labelFallback : label,
      value: formatEffectRefs(refs),
      labelHint: hasMpGated ? t("game.effectMpCostFallbackHint") || "被引用效果的魔法消耗不足时，同样触发此效果" : undefined
    })
  }
  return out
}
</script>

<style scoped>
@reference "#app-style";

.basic-attack-row {
  display: grid;
  gap: 2px;
}

.basic-attack-label {
  font-size: 12px;
  color: var(--app-muted);
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.basic-attack-label-hint {
  font-size: 12px;
  opacity: 0.75;
  cursor: help;
}

.basic-attack-value {
  font-size: 12px;
  color: var(--el-text-color-regular);
  white-space: pre-wrap;
  word-break: break-word;
}

.basic-attack-effect-block {
  border: 1px solid var(--app-border);
  border-radius: 8px;
  padding: 8px;
  background: color-mix(in srgb, var(--app-surface-elevated) 88%, var(--app-bg));
  color: var(--app-text);
}

.basic-attack-effect-block--compact {
  background: color-mix(in srgb, var(--app-surface-elevated) 88%, var(--app-bg));
}

.basic-attack-buff-list {
  @apply flex flex-wrap gap-1;
}

.basic-attack-toggle {
  display: flex;
  justify-content: flex-end;
}

.basic-attack-toggle-btn {
  padding-left: 0;
  padding-right: 0;
}

.basic-attack-toggle-count {
  margin-left: 4px;
  font-size: 11px;
  opacity: 0.75;
}
</style>
