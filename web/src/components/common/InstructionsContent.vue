<template>
  <div class="w-full">
    <component
      :is="activeComponent"
      :version="version"
      :effective-date="effectiveDate"
      :operator-name="operatorName"
      :contact-email="contactEmail"
      @accept="emit('accept')"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import InstructionCN from "./Instruction_zh.vue"
import InstructionEN from "./Instruction_en.vue"
import type { AppLocale } from "@/i18n"

const props = withDefaults(
  defineProps<{
    version?: string
    effectiveDate?: string
    operatorName?: string
    contactEmail?: string
  }>(),
  {
    version: "V1.0",
    effectiveDate: "请填写日期",
    operatorName: "请填写项目方名称",
    contactEmail: "example@example.com"
  }
)

const emit = defineEmits<{
  accept: []
}>()

const { locale } = useI18n()

const localeComponentMap: Record<AppLocale | "zh" | "en", typeof InstructionCN> = {
  "zh-CN": InstructionCN,
  "zh-TW": InstructionCN,
  zh: InstructionCN,
  "en-US": InstructionEN,
  ja: InstructionEN,
  ko: InstructionEN,
  ru: InstructionEN,
  en: InstructionEN
}

const activeComponent = computed(() => {
  const normalized = locale.value.trim()
  return localeComponentMap[normalized as keyof typeof localeComponentMap] ?? InstructionCN
})

const { version, effectiveDate, operatorName, contactEmail } = props
</script>
