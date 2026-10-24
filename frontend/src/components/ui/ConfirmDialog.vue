<script setup lang="ts">
import { Dialog } from '@ark-ui/vue/dialog'
import { useI18n } from '~/i18n'

// The app's one yes/no confirmation shell: an Ark Dialog with role="alertdialog",
// the cancel button on the left and the confirming one on the right (danger
// styling for anything destructive). The caller owns `open` and performs the
// action itself, so this component stays a dumb shell — it never closes on
// confirm, because the action usually has to await something first.
const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    description?: string
    confirmLabel: string
    /** Red confirm button + red border on hover: deleting, quitting, dropping. */
    danger?: boolean
    /** Set while the caller's action runs: both buttons disable and the dialog
     *  stops reacting to outside clicks / Escape, so it cannot vanish under the
     *  pointer mid-flight. */
    busy?: boolean
  }>(),
  { description: '', danger: false, busy: false },
)

const emit = defineEmits<{
  'update:open': [open: boolean]
  confirm: []
}>()

const { t } = useI18n()

function onOpenChange(open: boolean) {
  if (!open && props.busy) return
  emit('update:open', open)
}
</script>

<template>
  <Dialog.Root
    :open="props.open"
    role="alertdialog"
    lazy-mount
    unmount-on-exit
    @open-change="(details: unknown) => onOpenChange(!!(details as { open?: boolean })?.open)"
  >
    <Dialog.Backdrop class="dlg-back" />
    <Dialog.Positioner class="dlg-pos">
      <Dialog.Content class="dlg-card dlg-card--narrow">
        <Dialog.Title class="dlg-title">{{ props.title }}</Dialog.Title>
        <Dialog.Description v-if="props.description" class="dlg-desc">
          {{ props.description }}
        </Dialog.Description>
        <div class="dlg-actions confirm-actions">
          <button type="button" class="btn" :disabled="props.busy" @click="onOpenChange(false)">
            {{ t['cancel'] }}
          </button>
          <button
            type="button"
            class="btn"
            :class="props.danger ? 'btn--danger' : 'btn--primary'"
            :disabled="props.busy"
            @click="emit('confirm')"
          >
            {{ props.confirmLabel }}
          </button>
        </div>
      </Dialog.Content>
    </Dialog.Positioner>
  </Dialog.Root>
</template>

<style scoped>
/* `.dlg-desc` only carries a top margin, so the gap above the buttons is ours. */
.confirm-actions {
  margin-top: 16px;
}
</style>
