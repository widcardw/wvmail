<script setup lang="ts">
import { useI18n } from '~/i18n'
import { useElapsedSeconds } from '~/composables/use-elapsed-seconds'
import { EMAIL_PROVIDER_LABELS } from '~/types/email'
import type { EmailAccount, EmailProvider } from '~/types/email'

const props = defineProps<{
  accounts: EmailAccount[]
  testingId: string | null
  testSuccessId: string | null
  testFailId: string | null
  confirmingDeleteId: string | null
}>()

// Show how long the test has been running instead of a bare ellipsis: a
// connection that is going to time out is otherwise indistinguishable from one
// that is about to succeed.
const testElapsed = useElapsedSeconds(() => props.testingId !== null)

const emit = defineEmits<{
  add: []
  edit: [account: EmailAccount]
  test: [account: EmailAccount]
  delete: [account: EmailAccount]
}>()

const { t } = useI18n()

function providerLabel(provider: string): string {
  return EMAIL_PROVIDER_LABELS[provider as EmailProvider] || provider
}
</script>

<template>
  <div class="card">
    <header class="card__head">
      <h2>{{ t['settings.accounts'] }}</h2>
      <button type="button" class="btn btn--primary btn--sm" @click="emit('add')">
        {{ t['settings.accounts.add'] }}
      </button>
    </header>

    <p v-if="accounts.length === 0" class="empty">{{ t['settings.accounts.empty'] }}</p>
    <ul v-else class="list">
      <li v-for="account in accounts" :key="account.ID" class="row">
        <div class="row-info">
          <strong>{{ account.Name || account.Email }}</strong>
          <span class="chip">{{ providerLabel(account.Provider) }}</span>
          <div class="muted">
            {{ account.Email }} &middot; IMAP: {{ account.IMAPHost }}:{{ account.IMAPPort }}
          </div>
        </div>
        <div class="row-actions">
          <button
            type="button"
            class="drv"
            :class="{
              'drv--ok': testSuccessId === account.ID,
              'drv--bad': testFailId === account.ID,
              disabled: testingId === account.ID,
            }"
            :disabled="testingId === account.ID"
            @click="emit('test', account)"
          >
            {{ testingId === account.ID ? `${t['settings.accounts.testing']} ${testElapsed}s` : t['settings.accounts.test'] }}
          </button>
          <button type="button" class="drv" @click="emit('edit', account)">
            {{ t['settings.llm.edit'] }}
          </button>
          <button
            type="button"
            class="drv drv--danger"
            :disabled="testingId === account.ID"
            @click="emit('delete', account)"
          >
            {{
              confirmingDeleteId === account.ID
                ? t['settings.accounts.delete'] + '?'
                : t['settings.accounts.delete']
            }}
          </button>
        </div>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.card {
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
}
.card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-bottom: 1px solid var(--n-border-color);
}
.card__head h2 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}
.empty {
  padding: 30px 18px;
  color: var(--color-text-3);
  font-size: 13px;
}
.list {
  margin: 0;
  padding: 0;
  list-style: none;
}
.row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--n-border-color);
}
.row:last-child {
  border-bottom: none;
}
.row-info {
  min-width: 0;
}
.row-actions {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 4px;
}
.drv {
  padding: 3px 8px;
  color: var(--n-text-color);
  font-size: 12px;
  cursor: pointer;
  background: var(--n-action-color);
  border: 1px solid var(--n-border-color);
  border-radius: 5px;
}
.drv:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}
.drv--danger {
  color: #d03050;
}
.drv--ok {
  background: rgba(24, 160, 88, 0.2);
}
.drv--bad {
  background: rgba(208, 48, 80, 0.2);
}
@media (max-width: 560px) {
  .row {
    flex-direction: column;
  }
  .row-actions {
    width: 100%;
  }
}
</style>
