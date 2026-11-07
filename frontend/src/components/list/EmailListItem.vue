<script setup lang="ts">
import { computed } from 'vue'
import { formatDateRecent, displaySender, truncateSubject } from '~/composables/formats'
import { useI18n } from '~/i18n'
import { useLocaleStore } from '~/stores/locale'
import { useTagsStore } from '~/stores/tags'
import AccountAvatar from '~/components/icon/AccountAvatar.vue'
import TagChip from '~/components/tags/TagChip.vue'
import { isFlagged } from '~/composables/mail-flags'
import type { EmailSummary } from '~/types/email'

type ListEmail = EmailSummary & {
  account_id?: string
  folder_name?: string
}

const props = defineProps<{
  email: ListEmail
  selected: boolean
  smart: boolean
}>()

const emit = defineEmits<{
  select: [email: ListEmail]
}>()

/** Dates follow the app language, like every Ark component in the app. */
const localeStore = useLocaleStore()
const { tf } = useI18n()
const tagsStore = useTagsStore()
const visibleTags = computed(() =>
  (props.email.tag_ids ?? [])
    .map((id) => tagsStore.tagByID[id])
    .filter(Boolean)
    .slice(0, 3),
)
const hiddenTagCount = computed(() => Math.max(0, (props.email.tag_ids?.length ?? 0) - 3))
</script>

<template>
  <div
    class="email-item"
    :class="{ unread: !props.email.is_read, selected: props.selected }"
    @click="emit('select', props.email)"
  >
    <div class="email-row">
      <span class="email-body">
        <span class="email-header">
          <span v-if="props.smart || props.email.account_id" class="email-avatar">
            <AccountAvatar v-if="props.email.account_id" :account-id="props.email.account_id" />
          </span>
          <span class="email-sender">
            <span class="email-from">{{ displaySender(props.email) }}</span>
            <span
              v-if="(props.email.thread_count ?? 1) > 1"
              class="email-thread-count"
              :title="tf('mail.thread.count', { count: props.email.thread_count ?? 1 })"
            >
              {{ props.email.thread_count }}
            </span>
          </span>
          <i
            v-if="isFlagged(props.email.flags)"
            class="i-ri:star-fill email-star"
            aria-hidden="true"
          />
          <span class="email-date">{{
            formatDateRecent(props.email.date, localeStore.current)
          }}</span>
        </span>
        <span class="email-subject">{{ truncateSubject(props.email.subject) }}</span>
        <span v-if="visibleTags.length" class="email-tags">
          <TagChip v-for="tag in visibleTags" :key="tag.id" :tag="tag" compact />
          <span v-if="hiddenTagCount" class="email-tags__more">+{{ hiddenTagCount }}</span>
        </span>
      </span>
    </div>
  </div>
</template>

<style scoped>
.email-item {
  cursor: pointer;
  border-bottom: 1px solid var(--n-border-color);
  border-left: 2px solid transparent;
  color: var(--n-text-color);
}
.email-item:hover {
  background: var(--n-sidebar-hover);
}
.email-item.selected {
  border-left-color: var(--n-primary-color);
  background: var(--n-sidebar-active);
}
.email-row {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-width: 0;
  padding: 10px 12px;
  overflow: hidden;
}
.email-avatar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
}
.email-body {
  flex: 1;
  max-width: 100%;
  min-width: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.email-header {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}
.email-sender {
  display: flex;
  align-items: center;
  gap: 4px;
  flex: 1;
  min-width: 0;
}
.email-from {
  flex: 0 1 auto;
  min-width: 0;
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.email-date {
  font-size: 11px;
  opacity: 0.6;
  white-space: nowrap;
  flex-shrink: 0;
}
.email-star {
  flex: none;
  font-size: 12px;
  color: var(--color-warning);
}
.email-subject {
  display: -webkit-box;
  width: 100%;
  max-width: 100%;
  min-width: 0;
  font-size: 12px;
  opacity: 0.75;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 1;
  line-clamp: 1;
  margin-top: 2px;
}
.email-thread-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  border-radius: 8px;
  color: var(--color-text-3);
  background: var(--n-border-color);
  font-size: 10px;
  font-weight: 600;
  line-height: 1;
  vertical-align: 1px;
}
.email-tags {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  margin-top: 4px;
  overflow: hidden;
}
.email-tags__more {
  flex: none;
  color: var(--color-text-3);
  font-size: 10px;
}
.email-item.unread .email-from {
  font-weight: 700;
}
.email-item.unread .email-subject {
  font-weight: 600;
}
.email-item:not(.unread) .email-from {
  opacity: 0.85;
}
</style>
