<template>
  <div class="alchemy-submit">
    <div class="alchemy-submit__stats">
      <div v-if="goldCost > 0" class="alchemy-submit__stat">
        <el-popover placement="top" :width="260" trigger="hover" :show-after="120">
          <template #reference>
            <div class="alchemy-submit__label alchemy-submit__label--help">
              <span>{{ t("alchemy.cost.goldLabel") }}</span>
              <button type="button" class="alchemy-submit__help" :aria-label="t('alchemy.cost.goldLabel')" @click.stop>
                <Icon icon="ep:question-filled" />
              </button>
            </div>
          </template>
          <div class="alchemy-submit-popover">
            <div v-for="item in goldBreakdown" :key="`gold-${item.label}-${item.value}`" class="alchemy-submit-popover__row">
              <span>{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
            </div>
            <span v-if="goldBreakdown.length === 0" class="app-muted">{{ t("alchemy.cost.noBreakdown") }}</span>
          </div>
        </el-popover>
        <QuickRefWithRequire item-id="gold_coin" :required-count="goldCost" />
      </div>
      <div
        v-if="hasSuccessRate"
        class="alchemy-submit__stat"
        :style="successRateStyle"
      >
        <el-popover v-if="successBreakdown.length > 0" placement="top" :width="260" trigger="hover" :show-after="120">
          <template #reference>
            <div class="alchemy-submit__label alchemy-submit__label--help">
              <span>{{ t("alchemy.cost.successRateLabel") }}</span>
              <button type="button" class="alchemy-submit__help" :aria-label="t('alchemy.cost.successRateLabel')" @click.stop>
                <Icon icon="ep:question-filled" />
              </button>
            </div>
          </template>
          <div class="alchemy-submit-popover">
            <div v-for="item in successBreakdown" :key="`success-${item.label}-${item.value}`" class="alchemy-submit-popover__row">
              <span>{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
            </div>
          </div>
        </el-popover>
        <span v-else class="alchemy-submit__label">{{ t("alchemy.cost.successRateLabel") }}</span>
        <div class="alchemy-submit__value alchemy-submit__value--rate">{{ successRateText }}</div>
      </div>
      <div class="alchemy-submit__stat">
        <el-popover placement="top" :width="260" trigger="hover" :show-after="120">
          <template #reference>
            <div class="alchemy-submit__label alchemy-submit__label--help">
              <span>{{ t("alchemy.cost.durationLabel") }}</span>
              <button type="button" class="alchemy-submit__help" :aria-label="t('alchemy.cost.durationLabel')" @click.stop>
                <Icon icon="ep:question-filled" />
              </button>
            </div>
          </template>
          <div class="alchemy-submit-popover">
            <div v-for="item in durationBreakdown" :key="`duration-${item.label}-${item.value}`" class="alchemy-submit-popover__row">
              <span>{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
            </div>
            <span v-if="durationBreakdown.length === 0" class="app-muted">{{ t("alchemy.cost.noBreakdown") }}</span>
          </div>
        </el-popover>
        <div class="alchemy-submit__value">{{ durationText }}</div>
<slot name="duration-effect" />
      </div>
      <div class="alchemy-submit__stat">
        <el-popover placement="top" :width="260" trigger="hover" :show-after="120">
          <template #reference>
            <div class="alchemy-submit__label alchemy-submit__label--help">
              <span>{{ t("alchemy.cost.expLabel") }}</span>
              <button type="button" class="alchemy-submit__help" :aria-label="t('alchemy.cost.expLabel')" @click.stop>
                <Icon icon="ep:question-filled" />
              </button>
            </div>
          </template>
          <div class="alchemy-submit-popover">
            <div v-for="item in expBreakdown" :key="`exp-${item.label}-${item.value}`" class="alchemy-submit-popover__row">
              <span>{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
            </div>
            <span v-if="expBreakdown.length === 0" class="app-muted">{{ t("alchemy.cost.noBreakdown") }}</span>
          </div>
        </el-popover>
        <div class="alchemy-submit__value">{{ formatNumber(expReward, 2) }}</div>
      </div>
    </div>

    <div v-if="!lockCount" class="alchemy-submit__planner">
      <div class="alchemy-submit__count">
        <span class="alchemy-submit__label">{{ t("game.actionExecutionQA") }}</span>
        <el-input-number
          :model-value="count"
          :min="1"
          :max="maxCount"
          :step="1"
          size="small"
          controls-position="right"
          @update:model-value="updateCount"
        />
      </div>
      <div class="alchemy-submit__quick-group">
        <span class="alchemy-submit__label">{{ t("alchemy.cost.quickByCount") }}</span>
        <div class="alchemy-submit__quick alchemy-submit__quick--count">
          <el-button class="ml-0!" size="small" @click="appendCount(10)">+10</el-button>
          <el-button class="ml-0!" size="small" @click="appendCount(100)">+100</el-button>
          <el-button class="ml-0!" size="small" @click="appendCount(1000)">+1000</el-button>
        </div>
      </div>
      <div class="alchemy-submit__quick-group">
        <span class="alchemy-submit__label">{{ t("alchemy.cost.quickByTime") }}</span>
        <div class="alchemy-submit__quick alchemy-submit__quick--time">
          <el-button class="ml-0!" size="small" @click="appendByDuration(10 * 60)">{{ t("game.actionQuickAdd10m") }}</el-button>
          <el-button class="ml-0!" size="small" @click="appendByDuration(30 * 60)">{{ t("game.actionQuickAdd30m") }}</el-button>
          <el-button class="ml-0!" size="small" @click="appendByDuration(60 * 60)">{{ t("game.actionQuickAdd1h") }}</el-button>
          <el-button class="ml-0!" size="small" @click="appendByDuration(8 * 60 * 60)">{{ t("game.actionQuickAdd8h") }}</el-button>
        </div>
      </div>
    </div>

    <div class="alchemy-submit__buttons">
      <el-button class="alchemy-submit__button" type="primary" :disabled="disabled || maxCount <= 0" @click="runNow">
        {{ t("game.actionRunNow") }}
      </el-button>
      <el-button class="alchemy-submit__button" :disabled="disabled || maxCount <= 0" @click="enqueue">
        {{ t("game.actionEnqueue") }}
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import QuickRefWithRequire from "@components/game/item/QuickRefWithRequire.vue"
import { useActionSubmit } from "@composables/game/action/useActionSubmit"
import { formatDurationI18n } from "@/utils/duration-format"

type BreakdownItem = {
  label: string
  value: string
}

const props = withDefaults(defineProps<{
  actionId: string
  args?: Record<string, unknown>
  count: number
  maxCount?: number
  durationTotalSec?: number
  perRunDurationSec: number
  goldCost?: number
  expReward?: number
  successRate?: number | null
  disabled?: boolean
  goldBreakdown?: BreakdownItem[]
  durationBreakdown?: BreakdownItem[]
  expBreakdown?: BreakdownItem[]
  successBreakdown?: BreakdownItem[]
  lockCount?: boolean
}>(), {
  args: () => ({}),
  maxCount: 1,
  goldCost: 0,
  expReward: 0,
  successRate: null,
  disabled: false,
  goldBreakdown: () => [],
  durationBreakdown: () => [],
  expBreakdown: () => [],
  successBreakdown: () => [],
  lockCount: false,
})

const emit = defineEmits<{
  (e: "update:count", value: number): void
  (e: "submitted"): void
}>()

const { t } = useI18n()
const { runNowWithArgs, enqueueWithArgs } = useActionSubmit()

const normalizedMax = computed(() => Math.max(1, Math.floor(props.maxCount || 1)))
const durationText = computed(() => formatDurationI18n(props.durationTotalSec ?? Math.max(0, props.perRunDurationSec || 0) * Math.max(1, props.count || 1), t))
const rawSuccessRate = computed(() => Number(props.successRate))
const hasSuccessRate = computed(() => props.successRate !== null && props.successRate !== undefined && Number.isFinite(rawSuccessRate.value))
const normalizedSuccessRate = computed(() => hasSuccessRate.value ? Math.max(0, Math.min(1, rawSuccessRate.value)) : 0)
const successRateText = computed(() => `${(normalizedSuccessRate.value * 100).toFixed(2)}%`)
const successRateStyle = computed(() => {
  const progress = Math.max(0, Math.min(1, (normalizedSuccessRate.value - 0.10) / 0.80))
  const color = interpolateSuccessColor(progress)
  return {
    "--alchemy-success-color": color,
  }
})

function formatNumber(value: number, digits = 0): string {
  const n = Number(value || 0)
  return n.toLocaleString(undefined, {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })
}

function interpolateSuccessColor(progress: number): string {
  const red = [239, 68, 68]
  const orange = [245, 158, 11]
  const green = [34, 197, 94]
  const start = progress < 0.5 ? red : orange
  const end = progress < 0.5 ? orange : green
  const t = progress < 0.5 ? progress * 2 : (progress - 0.5) * 2
  const channels = start.map((value, index) => Math.round(value + (end[index] - value) * t))
  return `rgb(${channels[0]}, ${channels[1]}, ${channels[2]})`
}

function clampCount(value: number): number {
  return Math.min(normalizedMax.value, Math.max(1, Math.floor(value || 1)))
}

function updateCount(value: number | undefined) {
  emit("update:count", clampCount(Number(value || 1)))
}

function appendCount(delta: number) {
  updateCount((props.count || 1) + Math.max(1, Math.floor(delta || 1)))
}

function appendByDuration(seconds: number) {
  const perRun = Math.max(1, props.perRunDurationSec || 1)
  appendCount(Math.ceil(Math.max(1, seconds || 1) / perRun))
}

function runNow() {
  runNowWithArgs(props.actionId, clampCount(props.count), props.args, { actorType: "player", actorId: 0 })
  emit("submitted")
}

function enqueue() {
  enqueueWithArgs(props.actionId, clampCount(props.count), props.args, { actorType: "player", actorId: 0 })
  emit("submitted")
}
</script>

<style scoped>
.alchemy-submit {
  display: flex;
  flex-direction: column;
  gap: 14px;
  margin-top: 12px;
  padding: 16px 0px calc(16px + env(safe-area-inset-bottom, 0px));
  border-top: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--app-surface, #fff) 96%, var(--el-color-primary-light-9, #ecf5ff)), var(--app-surface, #fff));
}

.alchemy-submit__stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(132px, 1fr));
  gap: 10px;
  align-items: end;
}

.alchemy-submit__stat {
  display: grid;
  align-content: center;
  gap: 4px;
  min-width: 0;
  min-height: 64px;
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, var(--app-border, #dcdfe6) 70%, transparent);
  border-radius: 8px;
  background: color-mix(in srgb, var(--app-surface, #fff) 92%, transparent);
}

.alchemy-submit__label {
  font-size: 12px;
  color: var(--app-text-muted, #6b7280);
}

.alchemy-submit__label--help {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.alchemy-submit__help {
  display: inline-grid;
  place-items: center;
  width: 18px;
  height: 18px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: transparent;
  color: var(--app-muted, #6b7280);
  cursor: help;
}

.alchemy-submit__help:hover,
.alchemy-submit__help:focus-visible {
  color: var(--el-color-primary);
  background: color-mix(in srgb, var(--el-color-primary-light-9) 68%, transparent);
  outline: none;
}

.alchemy-submit__value {
  min-width: 84px;
  font-weight: 700;
  color: var(--el-color-primary);
}

.alchemy-submit__value--rate {
  color: var(--alchemy-success-color, var(--el-color-success));
}

.alchemy-submit__planner {
  display: grid;
  grid-template-columns: minmax(150px, 220px) minmax(210px, 0.8fr) minmax(300px, 1.2fr);
  gap: 12px;
  align-items: start;
}

.alchemy-submit__count {
  display: grid;
  gap: 6px;
}

.alchemy-submit__count :deep(.el-input-number) {
  width: 100%;
}

.alchemy-submit__quick-group {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.alchemy-submit__quick {
  display: grid;
  grid-template-columns: repeat(3, minmax(72px, 1fr));
  gap: 6px;
  min-width: 0;
}

.alchemy-submit__quick--time {
  grid-template-columns: repeat(4, minmax(82px, 1fr));
}

.alchemy-submit__quick :deep(.el-button) {
  min-width: 0;
  width: 100%;
}

.alchemy-submit__buttons {
  display: flex;
  gap: 10px;
  justify-content: start;
  /* max-width: calc(100% - 92px); */
}

.alchemy-submit__button {
  /* width: 100%; */
  font-weight: 700;
}

.alchemy-submit-popover {
  display: grid;
  gap: 6px;
  font-size: 12px;
}

.alchemy-submit-popover__row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

@media (max-width: 640px) {
  .alchemy-submit {
    gap: 14px;
    margin-inline: -2px;
    padding: 0px 0px calc(18px + env(safe-area-inset-bottom, 0px));
  }

  .alchemy-submit__stats {
    grid-template-columns: 1fr 1fr;
  }

  .alchemy-submit__planner {
    grid-template-columns: 1fr;
  }

  .alchemy-submit__quick {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .alchemy-submit__quick--time {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .alchemy-submit__buttons {
    position: sticky;
    bottom: calc(10px + env(safe-area-inset-bottom, 0px));
    z-index: 5;
    grid-template-columns: 1fr;
    max-width: none;
    box-sizing: border-box;
    background: color-mix(in srgb, var(--app-surface, #fff) 92%, transparent);
    backdrop-filter: blur(8px);

    margin-top: 1em;
  }

  .alchemy-submit__button {
    width: 100%;
    margin-left: 0;
    /* min-height: 44px; */
  }
}

@media (max-width: 420px) {
  .alchemy-submit__stats {
    grid-template-columns: 1fr;
  }

  .alchemy-submit__quick,
  .alchemy-submit__quick--time {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (min-width: 641px) and (max-width: 1180px) {
  .alchemy-submit__planner {
    grid-template-columns: minmax(150px, 220px) minmax(300px, 1fr);
  }

  .alchemy-submit__quick-group:last-child {
    grid-column: 1 / -1;
  }
}
</style>
