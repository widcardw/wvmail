<script setup lang="ts">
import { useI18n } from '~/i18n'
import { useLocaleStore } from '~/stores/locale'
import { formatDate } from '~/composables/formats'
import RecipientList from '~/components/detail/RecipientList.vue'
import UiIconAction from '~/components/ui/UiIconAction.vue'
import type { EmailDetail } from '~/types/email'

// The header of the reading pane: who sent it, when, the subject, the recipients,
// and the actions that act on the message as a whole (refresh, star, export, translate).
// The buttons receive their state as props rather than owning it.
// Recipients live in RecipientList: a big-company To/Cc list has to collapse.
const props = defineProps<{
  detail: EmailDetail
  loading: boolean
  exportingOriginal: boolean
  translating: boolean
  hasTranslation: boolean
  flagged: boolean
  flagging: boolean
}>()

const emit = defineEmits<{
  refresh: []
  'export-original': []
  'toggle-translation': []
  'toggle-flag': []
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
        <UiIconAction
          :label="t[flagged ? 'mail.flag.remove' : 'mail.flag.add']"
          :icon="flagged ? 'ri:star-fill' : 'ri:star-line'"
          :active="flagged"
          :busy="flagging"
          @click="emit('toggle-flag')"
        />
        <UiIconAction
          :label="t['mail.refresh.detail']"
          icon="ri:refresh-line"
          :disabled="loading"
          :busy="loading"
          @click="emit('refresh')"
        />
        <UiIconAction
          :label="t['mail.export.original']"
          icon="ri:download-2-line"
          :disabled="loading || exportingOriginal"
          :busy="exportingOriginal"
          @click="emit('export-original')"
        />
        <UiIconAction
          :label="translationTitle()"
          :icon="hasTranslation ? 'ri:close-line' : 'ri:translate-2'"
          :busy="translating"
          @click="emit('toggle-translation')"
        />
      </div>
    </div>

    <h2 class="subject">{{ detail.subject || t['mail.noSubject'] }}</h2>

    <div class="meta">
      <RecipientList
        v-if="(detail.to || []).length"
        :label="t['search.to']"
        :addresses="detail.to"
      />
      <RecipientList
        v-if="(detail.cc || []).length"
        :label="t['search.cc']"
        :addresses="detail.cc"
      />
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
.subject {
  font-size: 20px;
  font-weight: 700;
  margin: 12px 0 8px;
  line-height: 1.3;
  word-break: break-word;
}
.meta {
  font-size: 12px;
}
</style>
