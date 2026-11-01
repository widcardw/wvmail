<script setup lang="ts">
import { useI18n } from '~/i18n'
import UiIconAction from '~/components/ui/UiIconAction.vue'

defineProps<{
  hasDetail: boolean
  loading: boolean
  exportingOriginal: boolean
  translating: boolean
  hasTranslation: boolean
  flagged: boolean
  flagging: boolean
  deleting: boolean
}>()

const emit = defineEmits<{
  reply: []
  'reply-all': []
  forward: []
  refresh: []
  'export-original': []
  'toggle-translation': []
  'toggle-flag': []
  delete: []
}>()

const { t } = useI18n()
</script>

<template>
  <div class="action-bar" role="toolbar" :aria-label="t['mail.actions']">
    <div class="action-group">
      <UiIconAction
        :label="t['mail.reply']"
        icon="ri:reply-line"
        :disabled="!hasDetail"
        @click="emit('reply')"
      />
      <UiIconAction
        :label="t['mail.reply_all']"
        icon="ri:reply-all-line"
        :disabled="!hasDetail"
        @click="emit('reply-all')"
      />
      <UiIconAction
        :label="t['mail.forward']"
        icon="ri:share-forward-line"
        :disabled="!hasDetail"
        @click="emit('forward')"
      />
    </div>

    <span class="action-separator" aria-hidden="true" />

    <div class="action-group">
      <UiIconAction
        :label="t[flagged ? 'mail.flag.remove' : 'mail.flag.add']"
        :icon="flagged ? 'ri:star-fill' : 'ri:star-line'"
        :active="flagged"
        :busy="flagging"
        :disabled="!hasDetail"
        @click="emit('toggle-flag')"
      />

      <UiIconAction
        :label="t['mail.mark.unread']"
        icon="ri:mail-unread-line"
        :disabled="!hasDetail"
      />
    </div>

    <span class="action-separator" aria-hidden="true" />

    <div class="action-group">
      <UiIconAction
        :label="t['mail.refresh.detail']"
        icon="ri:refresh-line"
        :disabled="!hasDetail || loading"
        :busy="loading"
        @click="emit('refresh')"
      />
      <UiIconAction
        :label="t['mail.export.original']"
        icon="ri:download-2-line"
        :disabled="!hasDetail || loading || exportingOriginal"
        :busy="exportingOriginal"
        @click="emit('export-original')"
      />
      <UiIconAction
        :label="
          translating
            ? t['mail.translation.cancel']
            : hasTranslation
              ? t['mail.translation.close']
              : t['mail.translation.translate']
        "
        :icon="hasTranslation ? 'ri:close-line' : 'ri:translate-2'"
        :disabled="!hasDetail"
        :busy="translating"
        @click="emit('toggle-translation')"
      />
    </div>

    <div class="action-end">
      <UiIconAction
        :label="t['mail.delete']"
        icon="ri:delete-bin-5-line"
        danger
        :disabled="!hasDetail || deleting"
        :busy="deleting"
        @click="emit('delete')"
      />
    </div>
  </div>
</template>

<style scoped>
.action-bar {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 2px;
  height: var(--pane-header-height);
  padding: 0 12px;
  overflow-x: auto;
  overflow-y: hidden;
  background: var(--n-body-color);
  border-bottom: 1px solid var(--n-border-color);
  scrollbar-width: none;
}
.action-bar::-webkit-scrollbar {
  display: none;
}
.action-group,
.action-end {
  display: flex;
  flex: none;
  align-items: center;
  gap: 2px;
}
.action-end {
  margin-left: auto;
  padding-left: 8px;
}
.action-separator {
  flex: none;
  width: 1px;
  height: 18px;
  margin: 0 5px;
  background: var(--n-border-color);
}
</style>
