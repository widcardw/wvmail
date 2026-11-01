<script setup lang="ts">
import { Toast, Toaster } from '@ark-ui/vue/toast'
import { toaster } from '~/stores/feedback'
import { useI18n } from '~/i18n'

const { t } = useI18n()
</script>

<template>
  <Teleport to="body">
    <Toaster :toaster="toaster" v-slot="toast">
      <Toast.Root class="toast" :class="`toast--${toast.type}`">
        <Toast.Title class="toast__text">{{ toast.title }}</Toast.Title>
        <Toast.Description v-if="toast.description" class="toast__text">
          {{ toast.description }}
        </Toast.Description>
        <Toast.CloseTrigger type="button" class="toast__close" :aria-label="t['cancel']">
          <i class="i-ri:close-line" />
        </Toast.CloseTrigger>
      </Toast.Root>
    </Toaster>
  </Teleport>
</template>

<style scoped>
/* Ark owns the stack (placement, grouping, timers); only the card is ours.
   Sizing and the stacking variables live in styles/ui.css, because they must
   target Ark's `[data-part='root']` / `[data-part='group']` elements. */
.toast {
  pointer-events: auto;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 12px;
  font-size: 13px;
  color: var(--n-text-color);
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-left-width: 3px;
  border-radius: 6px;
  box-shadow: 0 6px 20px rgb(0 0 0 / 15%);
}
.toast__text {
  flex: 1;
  min-width: 0;
}
.toast--success {
  border-left-color: var(--color-success);
}
.toast--error {
  border-left-color: var(--color-danger);
}
.toast--warning {
  border-left-color: var(--color-warning);
}
.toast--info {
  border-left-color: var(--n-primary-color);
}
.toast__close {
  flex-shrink: 0;
  margin-left: auto;
  border: none;
  background: none;
  color: var(--n-text-color);
  opacity: 0.7;
  cursor: pointer;
}
.toast__close:hover {
  opacity: 1;
}
.toast__close i {
  font-size: 14px;
}
</style>
