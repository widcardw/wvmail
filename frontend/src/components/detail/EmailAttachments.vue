<script setup lang="ts">
import { Collapsible } from '@ark-ui/vue/collapsible'
import { useI18n } from '~/i18n'
import { useAccountsStore } from '~/stores/accounts'
import { useFeedbackStore } from '~/stores/feedback'
import type { AttachmentInfo } from '~/types/email'
import { getAttachmentIcon } from '~/composables/attachment-icons'
import { formatFileSize } from '~/composables/formats'

const props = defineProps<{
  attachments: AttachmentInfo[]
  open: boolean
}>()

const emit = defineEmits<{
  'update:open': [open: boolean]
}>()

const { t } = useI18n()
const accountsStore = useAccountsStore()
const fb = useFeedbackStore()

function onOpenChange(details: unknown) {
  const open = (details as { open?: boolean }).open
  if (typeof open === 'boolean') emit('update:open', open)
}

async function download(attachment: AttachmentInfo) {
  const path = await accountsStore.downloadAttachment(attachment)
  if (path) fb.success(`Downloaded → ${path}`)
}
</script>

<template>
  <Collapsible.Root
    data-side="attach"
    class="attach-wrap"
    :open="props.open"
    @open-change="onOpenChange"
  >
    <Collapsible.Trigger class="attach-head">
      <span class="chev" :class="{ open: props.open }">
        <i class="i-ri:arrow-right-s-line" />
      </span>
      <span>
        {{ props.attachments.length }}
        {{ props.attachments.length === 1 ? t['mail.attachment'] : t['mail.attachments'] }}
      </span>
    </Collapsible.Trigger>
    <Collapsible.Content class="attach-collapse">
      <div class="attach-body">
        <div
          v-for="attachment in props.attachments"
          :key="attachment.filename + '-' + attachment.size"
          class="attach-row"
        >
          <i
            :class="'i-' + getAttachmentIcon(attachment.mime_type, attachment.filename)"
            class="att-ic"
            aria-hidden="true"
          />
          <div class="att-info">
            <span class="att-name">
              {{ attachment.filename }}
              <span v-if="attachment.is_inline" class="muted-sm">
                ({{ t['mail.inline_image'] }})
              </span>
            </span>
            <span class="att-size">{{ formatFileSize(attachment.size) }}</span>
          </div>
          <button type="button" class="icon-btn" title="download" @click="download(attachment)">
            <i class="i-ri:download-2-line" aria-hidden="true" />
          </button>
        </div>
      </div>
    </Collapsible.Content>
  </Collapsible.Root>
</template>

<style scoped>
.attach-wrap {
  background: var(--n-action-color);
}
.attach-head {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 8px 16px;
  border: none;
  border-bottom: 1px solid var(--n-border-color);
  background: transparent;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
  color: inherit;
  text-align: left;
}
.chev {
  display: inline-flex;
  transition: transform 0.15s;
}
.chev.open {
  transform: rotate(90deg);
}
.attach-body {
  padding: 0;
}
.attach-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 20px;
  border-bottom: 1px solid var(--n-border-color);
}
.att-ic {
  font-size: 20px;
  flex-shrink: 0;
  color: var(--color-text-2);
}
.att-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.att-name {
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.att-size {
  font-size: 11px;
  opacity: 0.5;
}
.muted-sm {
  font-size: 11px;
  opacity: 0.55;
  margin-left: 2px;
}
</style>

<style>
/* Ark's generic content card styles should not affect this inline list. */
[data-side='attach'] [data-part='collapsible:content'],
[data-side='attach'] [data-scope='collapsible'][data-part='content'] {
  border: 0;
  border-radius: 0;
  background: transparent;
}
</style>
