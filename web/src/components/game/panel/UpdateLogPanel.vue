<template>
  <div class="space-y-4">
    <el-card class="app-card app-card--no-border game-page-frame" shadow="never">
      <template #header>
        <GamePageHeader
          title-key="game.updateLogTitle"
          :visual-src="updateLogVisual.src"
          :visual-tone="updateLogVisual.tone"
          visual-key="updateLog"
        />
      </template>

      <el-timeline class="md:pl-4! pl-0!">
        <el-timeline-item v-for="entry in localizedEntries" :key="entry.id" :timestamp="entry.date" placement="top" :type="entry.type">
          <div class="mb-2 flex items-center gap-2">
            <el-tag v-if="entry.version" size="small" type="info">{{ entry.version }}</el-tag>
          </div>
          <ul class="list-disc space-y-1 pl-4 text-sm">
            <li v-for="(lineEntry, idx) in entry.items" :key="`${entry.id}-${idx}`">
              <template v-for="(segment, segmentIdx) in lineEntry.segments" :key="`${entry.id}-${idx}-${segmentIdx}`">
                <template v-if="segment.kind === 'text'">{{ segment.text }}</template>
                <RouterLink v-else-if="segment.kind === 'menu'" class="update-log-menu-link" :to="segment.to">
                  <span>{{ segment.label }}</span>
                  <Icon class="update-log-menu-link__icon" icon="lucide:link-2" aria-hidden="true" />
                </RouterLink>
                <SkillQuickRef v-else-if="segment.kind === 'skill'" :skill-id="segment.id" />
                <EnemyUnitQuickRef v-else-if="segment.kind === 'enemy'" :enemy-id="segment.id" />
                <ActionQuickRef v-else-if="segment.kind === 'action'" :action-id="segment.id" />
                <template v-else-if="segment.kind === 'attr'">
                  <span class="inline-flex">
                    <StatusTag :attr-key="segment.id"></StatusTag>
                  </span>
                </template>
                <template v-else-if="segment.kind === 'affix'">
                  <span class="inline-flex">
                    <AffixQuickRef  :affix-id="segment.id" />
                  </span>
                </template>
                <QuickRef v-else :item-id="segment.id" />
              </template>
            </li>
          </ul>
        </el-timeline-item>
      </el-timeline>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { Icon } from "@iconify/vue"
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import { RouterLink } from "vue-router"
import { updateLogTimeline } from "@/data/update-log"
import { normalizeSupportedLocale } from "@/i18n"
import QuickRef from "@components/game/item/QuickRef.vue"
import SkillQuickRef from "@components/game/skill/SkillQuickRef.vue"
import EnemyUnitQuickRef from "@components/game/combat/EnemyUnitQuickRef.vue"
import AffixQuickRef from "../affix/AffixQuickRef.vue"
import ActionQuickRef from "../action/ActionQuickRef.vue"
import StatusTag from "../attribute/StatusTag.vue"
import GamePageHeader from "@components/game/common/GamePageHeader.vue"
import { gamePageVisuals } from "@services/game-page-visuals"
import { parseUpdateLogLineSegments } from "@services/update-log-content"

const { locale } = useI18n()
const updateLogVisual = gamePageVisuals.updateLog

const localeKey = computed<"zh-CN" | "en-US">(() => {
  const normalized = normalizeSupportedLocale(locale.value)
  return normalized === "en-US" ? "en-US" : "zh-CN"
})

const localizedEntries = computed(() =>
  updateLogTimeline.map((entry) => ({
    id: entry.id,
    type: 'primary',
    date: entry.date,
    version: entry.version || "",
    items: entry.items[localeKey.value].map((line) => ({
      segments: parseUpdateLogLineSegments(line)
    }))
  }))
)
</script>

<style scoped>
.update-log-menu-link {
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
  color: var(--el-color-primary);
  font-weight: 600;
  text-decoration-line: underline;
  text-decoration-thickness: 1px;
  text-underline-offset: 0.2em;
  transition: color 120ms ease;
}

.update-log-menu-link:hover {
  color: var(--el-color-primary-dark-2);
}

.update-log-menu-link:focus-visible {
  border-radius: 3px;
  outline: 2px solid var(--el-color-primary-light-3);
  outline-offset: 2px;
}

.update-log-menu-link__icon {
  width: 0.9em;
  height: 0.9em;
  flex: none;
}
</style>
