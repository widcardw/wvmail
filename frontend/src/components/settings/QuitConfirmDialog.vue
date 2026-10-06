<script setup lang="ts">
import { ref } from 'vue'
import { Dialog } from '@ark-ui/vue/dialog'
import { OsService } from '#/wvmail/services'
import { useFeedbackStore } from '~/stores/feedback'
import { useI18n } from '~/i18n'

// Confirmation for Settings → Quit. The action ends the process, so it gets a
// dialog with the destructive button on the right (and `role="alertdialog"`), and
// it is the only place in the app that calls OsService.QuitApp.
const props = defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  'update:open': [open: boolean]
}>()

const { t } = useI18n()
const feedback = useFeedbackStore()

/** Latched while the quit is in flight: the requests must not be doubled. */
const quitting = ref(false)

async function confirmQuit() {
  if (quitting.value) return
  quitting.value = true
  try {
    await OsService.QuitApp()
    // The call does not come back in practice — the process ends inside the
    // teardown — so the dialog deliberately stays up instead of flickering back to
    // an app that is already going away. The Go side exits on its own if AppKit
    // drops the terminate request, so this cannot leave a stuck window.
  } catch (e) {
    console.error('Failed to quit the app', e)
    quitting.value = false
    feedback.error(t.value['quitApp.failed'])
  }
}

function cancel() {
  if (quitting.value) return
  emit('update:open', false)
}
</script>

<template>
  <Dialog.Root
    :open="props.open"
    role="alertdialog"
    lazy-mount
    unmount-on-exit
    @open-change="
      (details: unknown) => emit('update:open', !!(details as { open?: boolean })?.open)
    "
  >
    <Dialog.Backdrop class="dlg-back" />
    <Dialog.Positioner class="dlg-pos">
      <Dialog.Content class="dlg-card dlg-card--narrow">
        <Dialog.Title class="dlg-title">{{ t['quitApp.title'] }}</Dialog.Title>
        <Dialog.Description class="dlg-desc">{{ t['quitApp.body'] }}</Dialog.Description>
        <div class="dlg-actions quit-actions">
          <button type="button" class="btn" :disabled="quitting" @click="cancel">
            {{ t['cancel'] }}
          </button>
          <button type="button" class="btn btn--danger" :disabled="quitting" @click="confirmQuit">
            {{ t['quitApp.confirm'] }}
          </button>
        </div>
      </Dialog.Content>
    </Dialog.Positioner>
  </Dialog.Root>
</template>

<style scoped>
/* The shared `.dlg-desc` only carries a top margin, so the gap above the actions
   is the dialog's own. */
.quit-actions {
  margin-top: 16px;
}
</style>
