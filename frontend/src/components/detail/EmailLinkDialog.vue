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
    @open-change="
      (details: unknown) => emit('update:open', !!(details as { open?: boolean })?.open)
    "
  >
    <Dialog.Backdrop class="d-back" />
    <Dialog.Positioner class="d-pos">
      <Dialog.Content class="d-card">
        <h3 class="d-title">{{ t['mail.link.title'] }}</h3>
        <p class="d-desc">{{ t['mail.link.body'] }}</p>
        <p class="d-url">{{ props.url }}</p>
        <div class="d-actions">
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
.d-back {
  position: fixed;
  inset: 0;
  z-index: 900;
  background: rgba(0, 0, 0, 0.4);
}
.d-pos {
  position: fixed;
  z-index: 901;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  pointer-events: none;
}
.d-card {
  pointer-events: auto;
  width: min(440px, 100%);
  background: var(--color-surface);
  color: var(--n-text-color);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-lg);
  padding: 16px;
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.25);
}
.d-title {
  margin: 0 0 6px;
  font-size: 15px;
}
.d-desc {
  margin: 0 0 8px;
  font-size: 13px;
  opacity: 0.85;
}
.d-url {
  margin: 0 0 16px;
  font-size: 12px;
  word-break: break-all;
  color: var(--info, #2080f0);
}
.d-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
