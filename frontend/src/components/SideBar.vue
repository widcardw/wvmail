<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NScrollbar, NMenu, NEmpty, NIcon } from 'naive-ui'
import { Icon } from '@iconify/vue'
import type { MenuOption } from 'naive-ui'
import { ICON_MAPPING } from '../composables/icon-mapping'
import { useI18n } from '../i18n'
import { useAccountsStore } from '../stores/accounts'
import { getFolderDisplay, compareFolders } from '../types/email'

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

const menuOptions = computed<MenuOption[]>(() => {
  const options: MenuOption[] = []

  for (const account of accountsStore.accounts) {
    const iconSource = ICON_MAPPING[account.Provider]
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
      icon: iconSource ? () => iconSource.create() : () => account.Email.charAt(0).toUpperCase(),
      children,
    })
  }

  return options
})

function handleUpdateValue(key: string, _item: MenuOption) {
  const parts = key.split(':')
  if (parts.length === 2) {
    const [accountId, folderName] = parts
    accountsStore.selectFolder(accountId, folderName)
    router.push('/')
  }
}
</script>

<template>
  <NScrollbar>
    <NEmpty
      v-if="accountsStore.accounts.length === 0"
      :description="t['settings.accounts.empty']"
      size="small"
      style="padding: 24px 12px"
    />
    <NMenu
      v-else
      :root-indent="16"
      :indent="16"
      :icon-size="20"
      :collapsed-icon-size="24"
      :expanded-keys="expandedKeys"
      :options="menuOptions"
      @update:value="handleUpdateValue"
      @update:expanded-keys="onExpandedKeysChange"
    />
  </NScrollbar>
</template>

<style scoped>
:deep(.n-menu-item-content--child .n-menu-item-content-header) {
  font-size: 13px;
}

:deep(.n-menu-item-content--child .n-menu-item-content__icon) {
  font-size: 16px;
}
</style>
