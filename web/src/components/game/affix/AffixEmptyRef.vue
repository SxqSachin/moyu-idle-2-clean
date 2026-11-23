<template>
  <div class="affix-quick-ref">
    <el-tag class="affix-quick-ref-tag w-fit" type="primary" effect="plain" :disable-transitions="true"
      :style="tagStyle">
      <span class="affix-quick-ref-content">
        <span v-if="prefixLabel">{{ prefixLabel }}</span>
      </span>
    </el-tag>

    <!-- <ul class="affix-quick-ref-lines list-disc list-inside">
      <li>{{ t("game.itemDetailAffixEmptyDetail")}}</li>
    </ul> -->
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue"
import { useI18n } from "vue-i18n"
import { useDevice } from "@composables/game/useDevice"
import TypeIcon from "@components/game/icon/TypeIcon.vue"
import { getRarityDefinition } from "@services/item-definition"
import type { EquipmentAffixSlot } from "@/types/equipment"

const props = withDefaults(
  defineProps<{
    slot?: EquipmentAffixSlot | null
    prefixLabel?: string
  }>(),
  {
    slot: null,
  }
)

const { t } = useI18n()
const { supportsHover } = useDevice()

const rarityID = computed(() => String(props.slot?.rarityId || "").trim())
const tagStyle = computed(() => {
  const color = rarityID.value ? getRarityDefinition(rarityID.value)?.color || "" : ""
  if (!color) return {}
  return {
    borderColor: color,
    color
  }
})
</script>

<style scoped>
.affix-quick-ref {
  display: grid;
  gap: 4px;
}

.affix-quick-ref-content {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.affix-quick-ref-lines {
  font-size: 12px;
  color: var(--el-text-color-regular);
  display: grid;
  gap: 2px;
}
</style>
