<template>
  <div class="alchemy-family-popover">
    <div class="alchemy-family-popover__title">{{ familyName }}</div>
    <div class="alchemy-family-popover__desc">{{ t("alchemy.familyDesc.default") }}</div>
    <div class="alchemy-family-popover__section">
      <div class="alchemy-family-popover__label">{{ t("alchemy.familyPopover.attrs") }}</div>
      <div class="alchemy-family-popover__chips">
        <el-tag v-for="attr in attrs" :key="attr" size="small" effect="plain">{{ t(attrNameKey(attr), attr) }}</el-tag>
      </div>
    </div>
    <div class="alchemy-family-popover__section">
      <div class="alchemy-family-popover__label">{{ t("alchemy.familyPopover.materials") }}</div>
      <div class="alchemy-family-popover__desc">{{ materialHint }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { attrNameKey, getAlchemyFamilyAttrs, ALCHEMY_FAMILY_MATERIAL_HINTS } from "@services/alchemy-codex"

const props = defineProps<{ family: string }>()
const { t } = useI18n()

const familyName = computed(() => t(`alchemy.family.${props.family}`, props.family))
const attrs = computed(() => getAlchemyFamilyAttrs(props.family))
const materialHint = computed(() => {
  const key = ALCHEMY_FAMILY_MATERIAL_HINTS[props.family] || "alchemy.familyMaterialHint.default"
  return t(key)
})
</script>

<style scoped>
.alchemy-family-popover {
  display: grid;
  gap: 8px;
  max-width: 320px;
}

.alchemy-family-popover__title {
  font-size: 14px;
  font-weight: 800;
  color: var(--app-text);
}

.alchemy-family-popover__desc {
  font-size: 12px;
  line-height: 1.45;
  color: var(--app-muted);
}

.alchemy-family-popover__section {
  display: grid;
  gap: 5px;
}

.alchemy-family-popover__label {
  font-size: 12px;
  font-weight: 700;
  color: var(--app-text);
}

.alchemy-family-popover__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}
</style>
