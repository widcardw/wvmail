<script setup lang="ts">
import { useI18n } from '~/i18n'
import EmailShadowBody from './EmailShadowBody.vue'
import UiAlert from '~/components/ui/UiAlert.vue'

// The AI assist result panels (translation HTML, plain text, or a Markdown
// summary) plus the error the service can return. Purely presentational: the
// parent decides whether the message may be shown at all.
defineProps<{
  /** Heading above the result, e.g. Translation or AI summary. */
  label: string
  /** Result body with layout, already sanitized. */
  html: string | null
  /** Result body as plain text (no layout to preserve). */
  text: string | null
  /** Service failure, shown as a dismissable alert. */
  error: string | null
  dark: boolean
}>()

const emit = defineEmits<{
  'dismiss-error': []
}>()

const { t } = useI18n()
</script>

<template>
  <div class="ai-assist">
    <div v-if="html" class="detail-body">
      <div class="td">{{ label }}</div>
      <EmailShadowBody :html="html" :dark="dark" />
    </div>
    <div v-if="text" class="detail-body">
      <div class="td">{{ label }}</div>
      <div class="body-text">{{ text }}</div>
    </div>
    <UiAlert
      v-if="error"
      class="ai-assist-alert"
      tone="error"
      dismissible
      :close-label="t['cancel']"
      @dismiss="emit('dismiss-error')"
    >
      {{ error }}
    </UiAlert>
  </div>
</template>

<style scoped>
.detail-body {
  padding: 18px 22px;
  line-height: 1.6;
  font-size: 14px;
  word-break: break-word;
}
.ai-assist-alert {
  margin: 12px;
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
</style>
