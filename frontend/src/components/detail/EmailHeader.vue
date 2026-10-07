<script setup lang="ts">
import { useI18n } from '~/i18n'
import { useLocaleStore } from '~/stores/locale'
import { formatDate } from '~/composables/formats'
import type { EmailDetail } from '~/types/email'

// The header of the reading pane: who sent it, when, the subject, the recipients,
// and the two actions that act on the message as a whole (re-fetch, translate).
// The translate button's label/icon is the whole translation state machine, so the
// three flags come in as props rather than the translation state itself.
const props = defineProps<{
  detail: EmailDetail
  loading: boolean
  translating: boolean
  hasTranslation: boolean
}>()

const emit = defineEmits<{
  refresh: []
  'toggle-translation': []
}>()

const { t } = useI18n()
const localeStore = useLocaleStore()

interface SenderParts {
  name: string | null
  addr: string | null
  fallback: string | null
}

/** Name+address when both are known, otherwise whatever single field we have. */
function senderDisplay(detail: {
  from_name?: string
  from_addr?: string
  from?: string
}): SenderParts {
  if (detail.from_name && detail.from_addr)
    return { name: detail.from_name, addr: detail.from_addr, fallback: null }
  if (detail.from_name) return { name: null, addr: null, fallback: detail.from_name }
  if (detail.from_addr) return { name: null, addr: null, fallback: detail.from_addr }
  if (detail.from) return { name: null, addr: null, fallback: detail.from }
  return { name: null, addr: null, fallback: 'Unknown' }
}

function translationTitle(): string {
  if (props.translating) return t.value['mail.translation.cancel']
  if (props.hasTranslation) return t.value['mail.translation.close']
  return t.value['mail.translation.translate']
}
</script>

<template>
  <div class="detail-header">
    <div class="head-row">
      <div class="sender">
        <span v-if="senderDisplay(detail).name" class="from">
          <strong>{{ senderDisplay(detail).name }}</strong>
          <span class="addr">&lt;{{ senderDisplay(detail).addr }}&gt;</span>
        </span>
        <span v-else class="from">{{ senderDisplay(detail).fallback }}</span>
        <span class="ddate">{{ formatDate(detail.date, localeStore.current) }}</span>
      </div>
      <div class="head-actions">
        <button
          type="button"
          class="icon-btn"
          :title="t['mail.refresh.detail']"
          :disabled="loading"
          @click="emit('refresh')"
        >
          <i class="i-ri:refresh-line" :class="{ rot: loading }" aria-hidden="true" />
        </button>
        <button
          type="button"
          class="icon-btn"
          :title="translationTitle()"
          @click="emit('toggle-translation')"
        >
          <span v-if="translating" class="ic-spin" />
          <i
            v-else
            :class="hasTranslation ? 'i-ri:close-line' : 'i-ri:translate-2'"
            aria-hidden="true"
          />
        </button>
      </div>
    </div>

    <h2 class="subject">{{ detail.subject || t['mail.noSubject'] }}</h2>

    <div class="meta">
      <div class="mrow">
        <span class="mlabel">{{ t['search.to'] }}</span>
        <span>{{ (detail.to || []).join(', ') }}</span>
      </div>
      <div v-if="detail.cc && detail.cc.length > 0" class="mrow">
        <span class="mlabel">Cc:</span>
        <span>{{ detail.cc.join(', ') }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.detail-header {
  padding: 18px 22px 14px;
  border-bottom: 1px solid var(--n-border-color);
}
.head-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}
.sender {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.from {
  font-size: 14px;
  font-weight: 600;
}
.addr {
  font-size: 13px;
  font-weight: 400;
  opacity: 0.65;
  margin-left: 4px;
}
.ddate {
  font-size: 12px;
  opacity: 0.6;
}
.head-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}
.rot {
  animation: rot360 0.8s linear infinite;
}
/* Top-of-detail actions become themed, visible buttons (not faint icons) */
.ic-spin {
  display: inline-block;
  width: 14px;
  height: 14px;
  border: 2px solid var(--n-border-color);
  border-top-color: var(--n-primary-color);
  border-radius: 50%;
  animation: rot360 0.8s linear infinite;
}
@keyframes rot360 {
  to {
    transform: rotate(360deg);
  }
}
.subject {
  font-size: 20px;
  font-weight: 700;
  margin: 12px 0 8px;
  line-height: 1.3;
  word-break: break-word;
}
.meta {
  font-size: 12px;
  opacity: 0.75;
}
.mrow {
  display: flex;
  gap: 6px;
  margin-bottom: 2px;
}
.mlabel {
  font-weight: 600;
  min-width: 26px;
  opacity: 0.6;
}
</style>
