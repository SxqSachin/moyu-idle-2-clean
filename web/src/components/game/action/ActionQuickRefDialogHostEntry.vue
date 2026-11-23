<template>
  <ActionDetailDialog
    v-if="actionDefinition"
    v-model="entry.visible"
    :action="actionDefinition"
    :count="entry.count"
    @update:count="onCountChange"
    @run-now="runNow"
    @enqueue="enqueue"
    @enqueue-plan="enqueuePlan"
  />
</template>

<script setup lang="ts">
import { computed } from "vue"
import ActionDetailDialog from "@components/game/action/ActionDetailDialog.vue"
import { getActionDefinition, isGenericActionVisible } from "@services/action-definition"
import { useActionSubmit } from "@/composables/game/action/useActionSubmit"
import { useActionQuickRefDialogStore, type ActionQuickRefDialogEntry } from "@stores/action-quick-ref-dialog"

const props = defineProps<{
  entry: ActionQuickRefDialogEntry
}>()

const dialogStore = useActionQuickRefDialogStore()
const { runNowWithArgs: submitRunNowWithArgs, enqueueWithArgs: submitEnqueueWithArgs, enqueuePlanBatch } = useActionSubmit()

const actionDefinition = computed(() => {
  const action = getActionDefinition(props.entry.actionId)
  return isGenericActionVisible(action) ? action : undefined
})

function onCountChange(value: number) {
  dialogStore.updateCount(props.entry.id, value)
}

function runNow(payload: { actionId: string; actorType: "player" | "kitty"; actorId: number; args?: Record<string, unknown> }) {
  submitRunNowWithArgs(payload.actionId, props.entry.count || 1, payload.args, {
    actorType: payload.actorType,
    actorId: payload.actorId
  })
  dialogStore.close(props.entry.id)
}

function enqueue(payload: { actionId: string; actorType: "player" | "kitty"; actorId: number; args?: Record<string, unknown> }) {
  submitEnqueueWithArgs(payload.actionId, props.entry.count || 1, payload.args, {
    actorType: payload.actorType,
    actorId: payload.actorId
  })
  dialogStore.close(props.entry.id)
}

async function enqueuePlan(payload: {
  steps: Array<{ actionId: string; count: number }>
  actorType: "player" | "kitty"
  actorId: number
  insertMode: "append" | "run_now_like_prepend"
}) {
  const result = await enqueuePlanBatch(payload.steps, {
    actorType: payload.actorType,
    actorId: payload.actorId
  }, {
    insertMode: payload.insertMode
  })
  if (result.ok) {
    dialogStore.close(props.entry.id)
  }
}
</script>
