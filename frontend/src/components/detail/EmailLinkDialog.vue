<script setup lang="ts">
import { Dialog } from '@ark-ui/vue/dialog'
import { OsService } from '#/wvmail/services'
import { useI18n } from '~/i18n'

const props = defineProps<{
  open: boolean
  url: string
}>()

const emit = defineEmits<{
  'update:open': [open: boolean]
}>()

const { t } = useI18n()

async function openLink() {
  emit('update:open', false)
  if (!props.url) return
  try {
    await OsService.OpenURL(props.url)
  } catch (e) {
    console.error('Failed to open URL', e)
  }
}
</script>

<template>
  <Dialog.Root
    :open="props.open"
    lazy-mount
    unmount-on-exit
    @open-change="
      (details: unknown) => emit('update:open', !!(details as { open?: boolean })?.open)
    "
  >
    <Dialog.Backdrop class="dlg-back" />
    <Dialog.Positioner class="dlg-pos">
      <Dialog.Content class="dlg-card">
        <h3 class="dlg-title">{{ t['mail.link.title'] }}</h3>
        <p class="dlg-desc">{{ t['mail.link.body'] }}</p>
        <p class="d-url">{{ props.url }}</p>
        <div class="dlg-actions">
          <button type="button" class="btn" @click="emit('update:open', false)">
            {{ t['cancel'] }}
          </button>
          <button type="button" class="btn btn--primary" @click="openLink">
            {{ t['mail.link.open'] }}
          </button>
        </div>
      </Dialog.Content>
    </Dialog.Positioner>
  </Dialog.Root>
</template>

<style scoped>
.d-url {
  margin: 0 0 16px;
  font-size: 12px;
  word-break: break-all;
  color: var(--info, #2080f0);
}
</style>
