<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '~/i18n'
import { formatDate } from '~/composables/formats'
import AddressActionsMenu from '~/components/detail/AddressActionsMenu.vue'
import RecipientList from '~/components/detail/RecipientList.vue'
import type { EmailDetail } from '~/types/email'

// The header of the reading pane: who sent it, when, the subject and the recipients.
// Message-wide actions live in the fixed bar above this scroll area.
// Recipients live in RecipientList: a big-company To/Cc list has to collapse.
const props = defineProps<{
  detail: EmailDetail
}>()

const { t, locale } = useI18n()

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

function extractAddress(raw: string): string {
  const match = raw.match(/<([^>]+)>/)
  return (match?.[1] || raw).trim()
}

const sender = computed(() => {
  const display = senderDisplay(props.detail)
  const raw = props.detail.from_addr || props.detail.from || display.addr || display.fallback || ''
  return {
    name:
      display.name || (display.fallback && !display.fallback.includes('@') ? display.fallback : ''),
    email: extractAddress(raw),
  }
})
</script>

<template>
  <div class="detail-header">
    <div class="sender">
      <AddressActionsMenu
        v-if="sender.email"
        :email="sender.email"
        :name="sender.name"
        variant="sender"
      >
        <span v-if="senderDisplay(detail).name" class="from">
          <strong>{{ senderDisplay(detail).name }}</strong>
          <span class="addr">&lt;{{ senderDisplay(detail).addr }}&gt;</span>
        </span>
        <span v-else class="from">{{ senderDisplay(detail).fallback }}</span>
      </AddressActionsMenu>
      <span v-else class="from">{{ senderDisplay(detail).fallback }}</span>
      <span class="ddate">{{ formatDate(detail.date, locale) }}</span>
    </div>

    <h2 class="subject">{{ detail.subject || t('mail.noSubject') }}</h2>

    <div class="meta">
      <RecipientList
        v-if="(detail.to || []).length"
        :label="t('search.to')"
        :addresses="detail.to"
      />
      <RecipientList
        v-if="(detail.cc || []).length"
        :label="t('search.cc')"
        :addresses="detail.cc"
      />
    </div>

    <slot name="after-meta" />
  </div>
</template>

<style scoped>
.detail-header {
  padding: 18px 22px 14px;
  border-bottom: 1px solid var(--n-border-color);
}
.sender {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
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
