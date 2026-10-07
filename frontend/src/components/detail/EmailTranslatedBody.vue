<script setup lang="ts">
import { useI18n } from '~/i18n'
import EmailShadowBody from './EmailShadowBody.vue'

// The translation result panels (HTML layout or plain text) plus the error the
// translator can return. Purely presentational: the parent decides whether the
// message may be shown at all and passes only what should be rendered.
defineProps<{
  /** Translated body with the original layout, already sanitized. */
  html: string | null
  /** Translated body as plain text (no layout to preserve). */
  text: string | null
  /** Translator failure, shown as a dismissable alert. */
  error: string | null
  dark: boolean
}>()

const emit = defineEmits<{
  'dismiss-error': []
}>()

const { t } = useI18n()
</script>

<template>
  <div class="translated">
    <div v-if="html" class="detail-body">
      <div class="td">{{ t['mail.translation.label'] }}</div>
      <EmailShadowBody :html="html" :dark="dark" />
    </div>
    <div v-if="text" class="detail-body">
      <div class="td">{{ t['mail.translation.label'] }}</div>
      <div class="body-text">{{ text }}</div>
    </div>
    <div v-if="error" class="alert alert--err with-x">
      <span>{{ error }}</span>
      <button type="button" class="x" aria-label="close" @click="emit('dismiss-error')">
        <i class="i-ri:close-line" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.detail-body {
  padding: 18px 22px;
  line-height: 1.6;
  font-size: 14px;
  word-break: break-word;
}
.body-text {
  font-family: inherit;
  white-space: pre-wrap;
}
.td {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  opacity: 0.5;
  margin-bottom: 8px;
}
.alert {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  margin: 12px;
  padding: 9px 12px;
  font-size: 13px;
  border-radius: 6px;
  border: 1px solid var(--n-border-color);
}
.alert--err {
  background: var(--color-danger-bg);
  border-color: var(--color-danger);
}
.with-x {
  align-items: center;
}
.x {
  border: none;
  background: none;
  cursor: pointer;
  color: inherit;
  opacity: 0.6;
}
.x i {
  font-size: 15px;
}
</style>
