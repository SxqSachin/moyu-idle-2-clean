<template>
  <section class="rounded-lg border border-[var(--app-border)] bg-[var(--app-surface)] p-4">
    <div class="mb-3 flex items-start justify-between gap-3 border-b border-[var(--app-border)] pb-3">
      <div class="min-w-0">
        <h2 class="truncate text-base font-extrabold text-[var(--app-text)]">{{ title }}</h2>
        <p v-if="subtitle" class="mt-0.5 text-xs text-[var(--app-muted)]">{{ subtitle }}</p>
      </div>
      <el-tag round effect="plain" size="small">{{ count }}</el-tag>
    </div>
    <div>
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
defineProps<{
  title: string
  subtitle?: string
  count: number
}>()
</script>
