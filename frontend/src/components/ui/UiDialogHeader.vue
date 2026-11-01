<script setup lang="ts">
import { Dialog } from '@ark-ui/vue/dialog'

/**
 * The standard title row for dialogs built on UiDialog. Keeping the close
 * trigger here makes the heading, spacing and accessible label consistent
 * without forcing every caller to import Ark's Dialog namespace.
 */
withDefaults(
  defineProps<{
    title: string
    closeLabel: string
    disabled?: boolean
  }>(),
  { disabled: false },
)

const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <header class="dlg-head">
    <Dialog.Title class="dlg-title">{{ title }}</Dialog.Title>
    <Dialog.CloseTrigger
      type="button"
      class="dlg-close"
      :aria-label="closeLabel"
      :disabled="disabled"
      @click="emit('close')"
    >
      <i class="i-ri:close-line" aria-hidden="true" />
    </Dialog.CloseTrigger>
  </header>
</template>
