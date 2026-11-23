<template>
  <el-tag
    :class="[
      'skill-quick-ref-tag',
      canOpenDialog ? 'skill-quick-ref-tag--clickable' : 'skill-quick-ref-tag--readonly'
    ]"
    :disable-transitions="true"
    effect="plain"
    :style="tagStyle"
    @click.stop="openDialog"
  >
    <ItemIcon v-if="showIcon && itemIconId" :item-id="itemIconId" :size="16" class="mr-1 inline-block align-[-0.12em]" />
    <Icon v-else-if="showIcon && skillIcon" :icon="skillIcon" class="mr-1 inline-block align-[-0.12em] text-[1em]" />
    <span v-if="prefixLabel">{{ prefixLabel }}</span>
    <span>{{ skillName }}</span>
    <template v-if="typeof remainingCooldown === 'number' && remainingCooldown > 0">
      <span>({{ t('game.battleUnitCooldown') }}: {{ remainingCooldown }})</span>
    </template>
  </el-tag>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { Icon } from "@iconify/vue"
import ItemIcon from "@components/game/icon/ItemIcon.vue"
import { getSkillDefinition, getSkillTypeDefinition, getLocalizedRuntimeSkillName } from "@services/skill-definition"
import { useSkillQuickRefDialogStore } from "@stores/skill-quick-ref-dialog"

const props = withDefaults(
  defineProps<{
    skillId: string
    remainingCooldown?: number | null
    totalCooldown?: number | null
    prefixLabel?: string
    showIcon?: boolean
    unitAttributes?: Record<string, number> | null
    unitSkillLevel?: number | null
    unitSkillExp?: number | null
    runtimeSkill?: Record<string, unknown> | null
  }>(),
  {
    remainingCooldown: null,
    totalCooldown: null,
    prefixLabel: "",
    showIcon: false,
    unitAttributes: null,
    unitSkillLevel: null,
    unitSkillExp: null,
    runtimeSkill: null
  }
)

const { t } = useI18n()
const dialogStore = useSkillQuickRefDialogStore()

const normalizedSkillID = computed(() => String(props.skillId || "").trim())
const skillDefinition = computed(() => getSkillDefinition(normalizedSkillID.value))
const skillIcon = computed(() => String(skillDefinition.value?.icon || "").trim())
const itemIconId = computed(() => skillIcon.value.startsWith("item.") ? skillIcon.value.slice("item.".length) : "")
const skillType = computed(() => getSkillTypeDefinition(skillDefinition.value?.category || ""))
const skillName = computed(() => getLocalizedRuntimeSkillName(t, normalizedSkillID.value, props.runtimeSkill))
const remainingCooldown = computed(() => {
  if (props.remainingCooldown === null || props.remainingCooldown === undefined) return null
  return Math.max(0, Math.trunc(Number(props.remainingCooldown || 0)))
})
const tagStyle = computed(() => {
  const color = skillType.value?.color || ""
  if (!color) return {}
  return {
    borderColor: color,
    color
  }
})
const prefixLabel = computed(() => props.prefixLabel || "")
const showIcon = computed(() => Boolean(props.showIcon))
const canOpenDialog = computed(() => normalizedSkillID.value.toLowerCase() !== "basic_attack")

function openDialog() {
  if (!canOpenDialog.value) return
  dialogStore.open({
    skillId: normalizedSkillID.value,
    remainingCooldown: remainingCooldown.value,
    totalCooldown: props.totalCooldown,
    unitAttributes: props.unitAttributes,
    unitSkillLevel: props.unitSkillLevel,
    unitSkillExp: props.unitSkillExp,
    runtimeSkill: props.runtimeSkill
  })
}
</script>

<style scoped>
.skill-quick-ref-tag--clickable {
  cursor: pointer;
}

.skill-quick-ref-tag--readonly {
  cursor: default;
}
</style>
