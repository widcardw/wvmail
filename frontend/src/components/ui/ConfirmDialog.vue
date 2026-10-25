<script setup lang="ts">
import { Dialog } from '@ark-ui/vue/dialog'
import { useI18n } from '~/i18n'
import UiDialog from '~/components/ui/UiDialog.vue'

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
</script>

<template>
  <UiDialog
    :open="props.open"
    size="narrow"
    role="alertdialog"
    :busy="props.busy"
    @update:open="emit('update:open', $event)"
  >
    <Dialog.Title class="dlg-title">{{ props.title }}</Dialog.Title>
    <Dialog.Description v-if="props.description" class="dlg-desc">
      {{ props.description }}
    </Dialog.Description>
    <!-- Optional body between the question and the buttons: an extra decision
         that belongs to this confirmation (e.g. "also delete cached mail"),
         so callers do not have to build their own dialog shell for it. -->
    <div v-if="$slots.extra" class="confirm-extra">
      <slot name="extra" />
    </div>
    <div class="dlg-actions confirm-actions">
      <button type="button" class="btn" :disabled="props.busy" @click="emit('update:open', false)">
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
  </UiDialog>
</template>

<style scoped>
/* `.dlg-desc` only carries a top margin, so the gap above the buttons is ours. */
.confirm-actions {
  margin-top: 16px;
}
.confirm-extra {
  margin-top: 14px;
}
</style>
