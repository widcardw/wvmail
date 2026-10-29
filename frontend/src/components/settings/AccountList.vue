<script setup lang="ts">
import { ref } from 'vue'
import { Menu } from '@ark-ui/vue/menu'
import { useI18n } from '~/i18n'
import { EMAIL_PROVIDER_LABELS } from '~/types/email'
import type { EmailAccount, EmailProvider } from '~/types/email'
import AccountAvatar from '~/components/icon/AccountAvatar.vue'

defineProps<{
  accounts: EmailAccount[]
  testingId: string | null
  testSuccessId: string | null
  testFailId: string | null
}>()

const emit = defineEmits<{
  add: []
  edit: [account: EmailAccount]
  test: [account: EmailAccount]
  move: [account: EmailAccount, offset: -1 | 1]
  delete: [account: EmailAccount]
}>()

const { t } = useI18n()

function providerLabel(provider: string): string {
  return EMAIL_PROVIDER_LABELS[provider as EmailProvider] || provider
}

/**
 * Which row's menu is open, by account id.
 *
 * The menu is controlled (rather than left to Ark) because one of its items must
 * not dismiss it: "test connection" keeps it open to show the spinner and then
 * the tick / cross, and those live inside the item.
 *
 * Everything else closes it explicitly — including the two move items, even
 * though it would be convenient to walk an account up several slots in one go:
 * the popup is positioned once, when it opens (zag's popper is created with
 * `listeners: false`), so a row that jumps to another slot would leave the menu
 * floating over the slot it used to occupy.
 */
const openMenuId = ref<string | null>(null)

function onOpenChange(account: EmailAccount, open: boolean) {
  if (open) {
    openMenuId.value = account.ID
  } else if (openMenuId.value === account.ID) {
    openMenuId.value = null
  }
}

function close() {
  openMenuId.value = null
}

function onSelect(account: EmailAccount, value: string | undefined) {
  switch (value) {
    case 'edit':
      close()
      emit('edit', account)
      break
    case 'test':
      // Deliberately stays open: the spinner and the result icon are rendered by
      // this row's item, and they would be destroyed the moment the menu unmounted.
      emit('test', account)
      break
    case 'up':
      close()
      emit('move', account, -1)
      break
    case 'down':
      close()
      emit('move', account, 1)
      break
    case 'delete':
      // The confirmation dialog is modal; leaving the menu open underneath it
      // would show a stray popup behind the backdrop.
      close()
      emit('delete', account)
      break
  }
}
</script>

<template>
  <section class="surface">
    <header class="surface__head">
      <h2>{{ t['settings.accounts'] }}</h2>
      <button type="button" class="btn btn--primary btn--sm" @click="emit('add')">
        {{ t['settings.accounts.add'] }}
      </button>
    </header>

    <p v-if="accounts.length === 0" class="empty">{{ t['settings.accounts.empty'] }}</p>
    <ul v-else class="list">
      <li v-for="(account, index) in accounts" :key="account.ID" class="row">
        <div class="row-main">
          <AccountAvatar :account-id="account.ID" :size="30" />
          <div class="row-info">
            <div class="flex items-center space-x-1">
              <strong>{{ account.Name || account.Email }}</strong>
              <span class="chip">{{ providerLabel(account.Provider) }}</span>
            </div>
            <div class="muted">
              {{ account.Email }} &middot; IMAP: {{ account.IMAPHost }}:{{ account.IMAPPort }}
            </div>
          </div>
        </div>

        <div class="row-actions">
          <Menu.Root
            :open="openMenuId === account.ID"
            :close-on-select="false"
            :positioning="{ placement: 'bottom-end', gutter: 6 }"
            lazy-mount
            unmount-on-exit
            @open-change="
              (details: unknown) => onOpenChange(account, !!(details as { open?: boolean })?.open)
            "
            @select="
              (details: unknown) =>
                onSelect(account, (details as { value?: string })?.value ?? undefined)
            "
          >
            <Menu.Trigger
              class="icon-btn row-menu-trigger"
              :aria-label="t['settings.accounts.actions']"
              :title="t['settings.accounts.actions']"
            >
              <i class="i-ri:more-2-line" aria-hidden="true" />
            </Menu.Trigger>

            <Teleport to="body">
              <Menu.Positioner class="acct-menu-pos">
                <Menu.Content class="acct-menu">
                  <Menu.Item value="edit" class="acct-menu__item">
                    <i class="i-ri:edit-2-line" aria-hidden="true" />
                    <Menu.ItemText>{{ t['settings.accounts.edit'] }}</Menu.ItemText>
                    <span class="acct-menu__state" aria-hidden="true" />
                  </Menu.Item>

                  <Menu.Item
                    value="test"
                    class="acct-menu__item"
                    :disabled="testingId === account.ID"
                  >
                    <i class="i-ri:plug-line" aria-hidden="true" />
                    <Menu.ItemText>{{ t['settings.accounts.test'] }}</Menu.ItemText>
                    <!-- Trailing slot: the same 16px box holds the spinner, then
                         the result, so the label never shifts sideways. -->
                    <span class="acct-menu__state" aria-hidden="true">
                      <span v-if="testingId === account.ID" class="acct-menu__spin" />
                      <i
                        v-else-if="testSuccessId === account.ID"
                        class="i-ri:check-line acct-menu__state--ok"
                      />
                      <i
                        v-else-if="testFailId === account.ID"
                        class="i-ri:close-line acct-menu__state--bad"
                      />
                    </span>
                  </Menu.Item>

                  <Menu.Separator class="acct-menu__sep" />

                  <!-- The first account has nothing above it and the last nothing
                       below it, so those items are absent rather than disabled. -->
                  <Menu.Item v-if="index > 0" value="up" class="acct-menu__item">
                    <i class="i-ri:arrow-up-line" aria-hidden="true" />
                    <Menu.ItemText>{{ t['settings.accounts.move_up'] }}</Menu.ItemText>
                    <span class="acct-menu__state" aria-hidden="true" />
                  </Menu.Item>
                  <Menu.Item
                    v-if="index < accounts.length - 1"
                    value="down"
                    class="acct-menu__item"
                  >
                    <i class="i-ri:arrow-down-line" aria-hidden="true" />
                    <Menu.ItemText>{{ t['settings.accounts.move_down'] }}</Menu.ItemText>
                    <span class="acct-menu__state" aria-hidden="true" />
                  </Menu.Item>

                  <Menu.Separator v-if="accounts.length > 1" class="acct-menu__sep" />

                  <Menu.Item value="delete" class="acct-menu__item acct-menu__item--danger">
                    <i class="i-ri:delete-bin-5-line" aria-hidden="true" />
                    <Menu.ItemText>{{ t['settings.accounts.delete'] }}</Menu.ItemText>
                    <span class="acct-menu__state" aria-hidden="true" />
                  </Menu.Item>
                </Menu.Content>
              </Menu.Positioner>
            </Teleport>
          </Menu.Root>
        </div>
      </li>
    </ul>
  </section>
</template>

<style scoped>
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
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--n-border-color);
}
.row:last-child {
  border-bottom: none;
}
.row-main {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.row-info {
  flex: 1;
  min-width: 0;
}
.row-actions {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 4px;
}
/* Base geometry comes from the shared .icon-btn; only the deltas live here. */
.row-menu-trigger {
  padding: 0;
}
.row-menu-trigger[data-state='open'] {
  background: var(--n-sidebar-hover);
}

/* The menu is portalled to <body>, so these rules cannot be scoped to the card:
   `scoped` still matches, because Vue stamps the child components' own root
   nodes — Menu.Positioner / Menu.Content are rendered from here, not by Ark. */
.acct-menu-pos {
  z-index: 1000;
}
.acct-menu {
  min-width: 172px;
  padding: 4px;
  color: var(--n-text-color);
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
  box-shadow: 0 8px 20px rgb(0 0 0 / 18%);
}
.acct-menu__item {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr) 16px;
  align-items: center;
  gap: 8px;
  width: 100%;
  height: 30px;
  padding: 0 8px;
  color: var(--n-text-color);
  font-size: 13px;
  background: transparent;
  border-radius: 4px;
  cursor: pointer;
  outline: none;
  user-select: none;
}
.acct-menu__item[data-highlighted] {
  background: var(--n-sidebar-hover);
}
.acct-menu__item[data-disabled] {
  cursor: not-allowed;
  opacity: 0.5;
}
.acct-menu__item--danger {
  color: var(--color-danger);
}
.acct-menu__item--danger[data-highlighted] {
  background: var(--color-danger-bg);
}
.acct-menu__item > i {
  font-size: 16px;
  line-height: 1;
}
.acct-menu__state {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  line-height: 1;
}
.acct-menu__state--ok {
  color: var(--color-success);
}
.acct-menu__state--bad {
  color: var(--color-danger);
}
/* The connection test can take seconds, so it gets a real spinner (the border
   trick from EmailHeader's .ic-spin) instead of a static icon. */
.acct-menu__spin {
  width: 12px;
  height: 12px;
  border: 1.5px solid var(--n-border-color);
  border-top-color: var(--n-primary-color);
  border-radius: 50%;
  animation: acct-spin 0.8s linear infinite;
}
@keyframes acct-spin {
  to {
    transform: rotate(360deg);
  }
}
.acct-menu__sep {
  margin: 4px 6px;
  border-top: 1px solid var(--n-border-color);
  color: var(--n-border-color);
  border-left: none;
  border-right: none;
  border-bottom: none;
}
@media (max-width: 560px) {
  .row {
    flex-direction: column;
  }
  .row-actions {
    width: 100%;
    justify-content: flex-end;
  }
}
</style>
