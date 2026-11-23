<template>
  <GameDialog v-model="visible" :title="t('game.destructiveRecycle.title')" width="520px"
    class="dialog-mobile-windowed" :close-on-click-modal="!submitting" :close-on-press-escape="!submitting" :show-close="!submitting">
    <div class="destructive-recycle">
      <el-alert :title="t('game.destructiveRecycle.warning')" type="warning" :closable="false" show-icon />
      <div v-if="loading" role="status">{{ t('game.destructiveRecycle.loading') }}</div>
      <el-alert v-else-if="errorMessage" :title="errorMessage" type="error" :closable="false" show-icon />
      <template v-if="quote">
        <div ref="listElement" class="destructive-recycle__items">
          <div v-for="item in visibleItems" :key="item.instanceId" class="destructive-recycle__item">
            <Card :item-id="item.templateItemId" :width="40" :height="40" hide-label />
            <div class="destructive-recycle__description">
              <div>{{ getLocalizedItemName(t, item.templateItemId, { fallbackText: item.templateItemId }) }}<span v-if="item.enhanceLevel > 0" class="destructive-recycle__enhance"> +{{ item.enhanceLevel }}</span></div>
              <div class="destructive-recycle__affixes">
                <AffixQuickRef v-for="slot in item.affixes" :key="slot.slotIndex" :affix-id="slot.affixId" :slot="slot" />
              </div>
            </div>
            <QuickRef item-id="gold_coin" :count="item.totalGold" />
          </div>
        </div>
        <el-pagination v-if="quote.items.length > pageSize" v-model:current-page="page" :page-size="pageSize" :total="quote.items.length" :pager-count="5" layout="prev, pager, next" size="small" />
        <dl class="destructive-recycle__totals">
          <div><dt>{{ t('game.destructiveRecycle.bodyValue') }}</dt><dd><QuickRef item-id="gold_coin" :count="quote.bodyGold" /></dd></div>
          <div><dt>{{ t('game.destructiveRecycle.fee', { percent: quote.feePercent }) }}</dt><dd>-<QuickRef item-id="gold_coin" :count="quote.disposalFee" /></dd></div>
          <div class="destructive-recycle__net"><dt>{{ t('game.recycleTotalGoldLabel') }}</dt><dd><QuickRef item-id="gold_coin" :count="quote.totalGold" /></dd></div>
        </dl>
        <el-checkbox v-model="acknowledged" :disabled="submitting" class="destructive-recycle__ack">
          {{ t('game.destructiveRecycle.acknowledge', { items: quote.items.length, affixes: affixCount, enhanced: enhancedCount }) }}
        </el-checkbox>
      </template>
    </div>
    <template #footer>
      <el-button :disabled="submitting" @click="visible = false">{{ t('game.cancel') }}</el-button>
      <el-button v-if="!quote" :loading="loading" :disabled="loading || submitting || !items.length" @click="loadQuote">
        <Icon icon="lucide:refresh-cw" />{{ t('game.destructiveRecycle.refresh') }}
      </el-button>
      <el-button v-else type="danger" :loading="submitting" :disabled="!acknowledged || loading || submitting" @click="confirmRecycle">
        <Icon icon="lucide:trash-2" />{{ t('game.destructiveRecycle.confirm') }}
      </el-button>
    </template>
  </GameDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue"
import { ElMessage } from "element-plus"
import { Icon } from "@iconify/vue"
import { useI18n } from "vue-i18n"
import { apiPost } from "@services/api"
import { resolveErrorMessage } from "@services/error-messages"
import { getLocalizedItemName } from "@services/item-label"
import { destructiveRecycleSelectionKey, type DestructiveRecycleQuote } from "@services/destructive-recycle"
import { useGameStore } from "@stores/game"
import type { InventoryInstance } from "@/types/equipment"
import GameDialog from "@components/game/common/GameDialog.vue"
import Card from "./Card.vue"
import QuickRef from "./QuickRef.vue"
import AffixQuickRef from "@components/game/affix/AffixQuickRef.vue"

const props = defineProps<{ modelValue: boolean; items: InventoryInstance[] }>()
const emit = defineEmits<{ (e: "update:modelValue", value: boolean): void; (e: "done"): void }>()
const { t } = useI18n()
const gameStore = useGameStore()
const loading = ref(false)
const submitting = ref(false)
const acknowledged = ref(false)
const errorMessage = ref("")
const quote = ref<DestructiveRecycleQuote | null>(null)
const pageSize = 20
const page = ref(1)
const listElement = ref<HTMLElement | null>(null)
const visibleItems = computed(() => quote.value?.items.slice((page.value - 1) * pageSize, page.value * pageSize) || [])
const selectionKey = computed(() => destructiveRecycleSelectionKey(props.items))
const affixCount = computed(() => quote.value?.items.reduce((n, item) => n + item.affixes.length, 0) || 0)
const enhancedCount = computed(() => quote.value?.items.filter((item) => item.enhanceLevel > 0).length || 0)
let requestVersion = 0
let dialogSession = 0
let mounted = true
let quotedKey = ""
let quotedPrincipal = ""
const visible = computed({ get: () => props.modelValue, set: (value: boolean) => { if (!submitting.value) emit("update:modelValue", value) } })

function invalidate() {
  requestVersion++
  loading.value = false
  quote.value = null
  acknowledged.value = false
  quotedKey = ""
  quotedPrincipal = ""
  page.value = 1
}

watch(page, () => { if (listElement.value) listElement.value.scrollTop = 0 })

watch(() => props.modelValue, (open) => {
  dialogSession++
  invalidate()
  errorMessage.value = ""
  if (open) void loadQuote()
}, { immediate: true })
watch([selectionKey, () => gameStore.token], () => {
  invalidate()
  if (props.modelValue) errorMessage.value = t("game.destructiveRecycle.changed")
})
onBeforeUnmount(() => { mounted = false; invalidate() })

async function loadQuote() {
  if (submitting.value || !props.modelValue || !gameStore.token || !props.items.length) return
  invalidate()
  const version = requestVersion
  const principal = gameStore.token
  const key = selectionKey.value
  loading.value = true
  errorMessage.value = ""
  try {
    const result = await apiPost<DestructiveRecycleQuote>("/api/items/recycle/destructive/quote", principal, {
      items: props.items.map((item) => ({ instanceId: item.instanceId }))
    })
    if (version !== requestVersion || !props.modelValue || gameStore.token !== principal || selectionKey.value !== key) return
    quote.value = result
    quotedKey = key
    quotedPrincipal = principal
  } catch (err) {
    if (version === requestVersion) errorMessage.value = resolveErrorMessage(t, err, "game.recycleFailed")
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

async function confirmRecycle() {
  if (submitting.value || loading.value || !acknowledged.value || !quote.value || !props.modelValue) return
  if (selectionKey.value !== quotedKey || gameStore.token !== quotedPrincipal) {
    invalidate()
    errorMessage.value = t("game.destructiveRecycle.changed")
    return
  }
  const principal = quotedPrincipal
  const session = dialogSession
  const items = quote.value.items.map((item) => ({ instanceId: item.instanceId, quoteToken: item.quoteToken }))
  submitting.value = true
  try {
    const result = await apiPost<DestructiveRecycleQuote>("/api/items/recycle/destructive", principal, { items })
    if (!mounted || gameStore.token !== principal || session !== dialogSession) return
    ElMessage.success(t("game.batchRecycleSuccess", { gold: result.totalGold }))
    emit("update:modelValue", false)
    emit("done")
  } catch (err) {
    // Even an uncertain response requires a fresh preview and explicit consent,
    // never a replay of an irreversible mutation.
    if (!mounted || session !== dialogSession) return
    invalidate()
    if (gameStore.token === principal) errorMessage.value = resolveErrorMessage(t, err, "game.recycleFailed")
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.destructive-recycle { display: grid; gap: 14px; }
.destructive-recycle__items { max-height: 280px; overflow-y: auto; }
.destructive-recycle__item { display: grid; grid-template-columns: 40px minmax(0, 1fr) auto; gap: 10px; align-items: start; padding: 10px 0; border-bottom: 1px solid var(--app-border); }
.destructive-recycle__description { min-width: 0; overflow-wrap: anywhere; font-size: 13px; }
.destructive-recycle__enhance { white-space: nowrap; font-weight: 600; color: var(--el-color-warning); }
.destructive-recycle__affixes { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 6px; }
.destructive-recycle__affixes :deep(.affix-quick-ref) { max-width: 100%; }
.destructive-recycle__affixes :deep(.el-tag) { max-width: 100%; height: auto; white-space: normal; }
.destructive-recycle__totals { margin: 0; display: grid; gap: 8px; font-size: 13px; }
.destructive-recycle__totals > div { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 4px 12px; }
.destructive-recycle__totals dd { margin: 0; display: flex; align-items: center; }
.destructive-recycle__net { padding-top: 8px; border-top: 1px solid var(--app-border); font-weight: 600; }
.destructive-recycle__ack { height: auto; align-items: flex-start; white-space: normal; }
.destructive-recycle__ack :deep(.el-checkbox__input) { margin-top: 3px; }
.destructive-recycle__ack :deep(.el-checkbox__label) { white-space: normal; overflow-wrap: anywhere; line-height: 1.5; }
</style>
