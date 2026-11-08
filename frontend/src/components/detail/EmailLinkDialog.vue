<script setup lang="ts">
import { Dialog } from '@ark-ui/vue/dialog'
import { OsService } from '#/wvmail/services'
import { useI18n } from '~/i18n'
import UiDialog from '~/components/ui/UiDialog.vue'

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
  <UiDialog
    :open="props.open"
    :close-on-interact-outside="false"
    padded
    @update:open="emit('update:open', $event)"
  >
    <Dialog.Title class="dlg-title">{{ t('mail.link.title') }}</Dialog.Title>
    <Dialog.Description class="dlg-desc">{{ t('mail.link.body') }}</Dialog.Description>
    <p class="d-url">{{ props.url }}</p>
    <div class="dlg-actions">
      <button type="button" class="btn" @click="emit('update:open', false)">
        {{ t('cancel') }}
      </button>
      <button type="button" class="btn btn--primary" @click="openLink">
        {{ t('mail.link.open') }}
      </button>
    </div>
  </UiDialog>
</template>

<style scoped>
.d-url {
  margin: 0 0 16px;
  font-size: 12px;
  word-break: break-all;
  color: var(--info, #2080f0);
}
</style>
