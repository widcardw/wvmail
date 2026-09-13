<script setup lang="ts">
import { formatDateRecent, displaySender, truncateSubject } from '~/composables/formats'
import AccountAvatar from './AccountAvatar.vue'
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
          <span class="email-from">{{ displaySender(props.email) }}</span>
          <span class="email-date">{{ formatDateRecent(props.email.date) }}</span>
        </span>
        <span class="email-subject">{{ truncateSubject(props.email.subject) }}</span>
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
.email-from {
  font-size: 13px;
  font-weight: 500;
  flex: 1;
  min-width: 0;
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
.email-subject {
  font-size: 12px;
  opacity: 0.75;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-top: 2px;
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
