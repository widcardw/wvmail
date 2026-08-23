<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NScrollbar, NMenu, NEmpty, NIcon } from 'naive-ui'
import { Icon } from '@iconify/vue'
import type { MenuOption } from 'naive-ui'
import { useI18n } from '../i18n'
import { useAccountsStore } from '../stores/accounts'
import { getFolderDisplay, compareFolders } from '../types/email'
import AccountAvatar from './AccountAvatar.vue'

const props = defineProps<{
  collapsed?: boolean
}>()

const { t, locale } = useI18n()
const accountsStore = useAccountsStore()
const router = useRouter()

const expandedKeys = ref<string[]>([])

onMounted(() => {
  if (accountsStore.accounts.length === 0) {
    accountsStore.loadAccounts()
  }
})

function renderIconifyIcon(iconName: string) {
  return () => h(NIcon, null, { default: () => h(Icon, { icon: iconName }) })
}

function onExpandedKeysChange(keys: string[]) {
  for (const key of keys) {
    if (!expandedKeys.value.includes(key)) {
      const account = accountsStore.accounts.find((a) => a.ID === key)
      if (account && !accountsStore.foldersByAccount[key]) {
        accountsStore.loadFolders(account)
      }
    }
  }
  expandedKeys.value = keys
}

const smartFolderOptions = computed<MenuOption[]>(() => {
  const smartFolderTypes: { key: string; tKey: string; icon: string }[] = [
    { key: 'inbox', tKey: 'sidebar.all_inbox', icon: 'ri:inbox-archive-line' },
    { key: 'sent', tKey: 'sidebar.all_sent', icon: 'ri:send-plane-2-line' },
    { key: 'drafts', tKey: 'sidebar.all_drafts', icon: 'ri:draft-line' },
  ]

  return smartFolderTypes.map((sf) => ({
    label: t.value[sf.tKey],
    key: `__smart__:${sf.key}`,
    icon: renderIconifyIcon(sf.icon),
  }))
})

const accountMenuOptions = computed<MenuOption[]>(() => {
  const options: MenuOption[] = []
  for (const account of accountsStore.accounts) {
    const folders = accountsStore.foldersByAccount[account.ID]
    const isLoading = folders === undefined

    let children: MenuOption[] | undefined

    if (isLoading) {
      children = [
        {
          label: 'Loading...',
          key: `${account.ID}:__loading__`,
          icon: renderIconifyIcon('eos-icons:loading'),
          disabled: true,
        },
      ]
    } else if (folders && folders.length > 0) {
      const filtered = folders.filter(
        (f) => !f.attrs.some((a) => a.includes('NoSelect') || a.includes('NonExistent')),
      )

      if (filtered.length === 0) {
        children = folders.map((f) => ({
          label: f.name,
          key: `${account.ID}:${f.name}`,
          icon: renderIconifyIcon('ri:folder-2-fill'),
        }))
      } else {
        const sorted = [...filtered].sort((a, b) => compareFolders(a, b, locale.value))
        children = sorted.map((f) => {
          const display = getFolderDisplay(f.name, locale.value)
          return {
            label: display.label,
            key: `${account.ID}:${f.name}`,
            icon: renderIconifyIcon(display.icon),
          }
        })
      }
    }

    options.push({
      label: account.Name || account.Email,
      key: account.ID,
      icon: () => h(AccountAvatar, { accountId: account.ID, size: 18 }),
      children,
    })
  }

  return options
})

const utilityMenuOptions = computed<MenuOption[]>(() => [
  {
    label: t.value['settings.common'],
    key: 'settings',
    icon: renderIconifyIcon('ri:settings-2-line'),
  },
  {
    label: t.value['reminders.title'],
    key: 'reminder',
    icon: renderIconifyIcon('ri:notification-2-line'),
  },
  {
    label: t.value['notes.title'],
    key: 'notes',
    icon: renderIconifyIcon('ri:pen-nib-fill'),
  },
])

const activeUtilityKey = computed(() => {
  const path = router.currentRoute.value.path
  if (path.startsWith('/settings')) return 'settings'
  if (path.startsWith('/reminder')) return 'reminder'
  if (path.startsWith('/notes')) return 'notes'
  return null
})

function handleUpdateValue(key: string, _item: MenuOption) {
  const parts = key.split(':')
  if (parts.length === 2) {
    if (parts[0] === '__smart__') {
      accountsStore.selectSmartFolder(parts[1])
      router.push('/')
      return
    }
    const [accountId, folderName] = parts
    accountsStore.selectFolder(accountId, folderName)
    router.push('/')
  }
}

function handleUtilityClick(key: string) {
  const paths: Record<string, string> = {
    settings: '/settings',
    reminder: '/reminder',
    notes: '/notes',
  }
  const path = paths[key]
  if (path) router.push(path)
}
</script>

<template>
  <div class="sidebar-content">
    <div class="sidebar-overview">
      <NMenu
        :collapsed="props.collapsed"
        :root-indent="16"
        :indent="16"
        :icon-size="20"
        :collapsed-icon-size="24"
        :theme-overrides="{ itemHeight: '32px' }"
        :options="smartFolderOptions"
        @update:value="handleUpdateValue"
      />
    </div>

    <div class="sidebar-divider" />

    <div class="sidebar-accounts">
      <NScrollbar style="height: 100%">
        <NEmpty
          v-if="accountsStore.accounts.length === 0"
          :description="t['settings.accounts.empty']"
          size="small"
          style="padding: 24px 12px"
        />
        <NMenu
          v-else
          :collapsed="props.collapsed"
          :root-indent="16"
          :indent="16"
          :icon-size="20"
          :collapsed-icon-size="24"
          :theme-overrides="{ itemHeight: '32px' }"
          :expanded-keys="expandedKeys"
          :options="accountMenuOptions"
          @update:value="handleUpdateValue"
          @update:expanded-keys="onExpandedKeysChange"
        />
      </NScrollbar>
    </div>

    <div class="sidebar-divider" />

    <div class="sidebar-tools">
      <NMenu
        :value="activeUtilityKey"
        :collapsed="props.collapsed"
        :root-indent="16"
        :indent="16"
        :icon-size="20"
        :collapsed-icon-size="24"
        :theme-overrides="{ itemHeight: '32px' }"
        :options="utilityMenuOptions"
        @update:value="handleUtilityClick"
      />
    </div>
  </div>
</template>

<style scoped>
.sidebar-content {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.sidebar-overview,
.sidebar-tools {
  flex-shrink: 0;
}

.sidebar-accounts {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.sidebar-divider {
  height: 1px;
  margin: 4px 12px;
  background: var(--n-border-color);
  opacity: 0.7;
}

:deep(.n-menu-item-content--child .n-menu-item-content-header) {
  font-size: 13px;
}

:deep(.n-menu-item-content--child .n-menu-item-content__icon) {
  font-size: 16px;
}
</style>
