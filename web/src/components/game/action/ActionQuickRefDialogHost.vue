<template>
  <!-- Keep the dialog stack outside hover popovers so nested refs do not get unmounted. -->
  <ActionQuickRefDialogHostEntry
    v-for="entry in dialogStore.entries"
    :key="entry.id"
    :entry="entry"
  />
</template>

<script setup lang="ts">
import ActionQuickRefDialogHostEntry from "@components/game/action/ActionQuickRefDialogHostEntry.vue"
import { useActionQuickRefDialogStore } from "@stores/action-quick-ref-dialog"

const dialogStore = useActionQuickRefDialogStore()
</script>
