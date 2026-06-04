<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NScrollbar, NMenu, NEmpty } from 'naive-ui'
import type { MenuOption } from 'naive-ui'
import { ICON_MAPPING } from '../composables/icon-mapping'
import { useI18n } from '../i18n'
import { useAccountsStore } from '../stores/accounts'
import { getFolderDisplay } from '../types/email'

const { t } = useI18n()
const accountsStore = useAccountsStore()

const expandedKeys = ref<string[]>([])

onMounted(() => {
  if (accountsStore.accounts.length === 0) {
    accountsStore.loadAccounts()
  }
})

function onExpandedKeysChange(keys: string[]) {
  // Find newly expanded accounts
  for (const key of keys) {
    if (!expandedKeys.value.includes(key)) {
      const account = accountsStore.accounts.find(a => a.ID === key)
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
      // Not loaded yet — placeholder will be replaced once loaded
      children = [
        {
          label: 'Loading...',
          key: `${account.ID}:__loading__`,
          icon: () => '⏳',
          disabled: true,
        },
      ]
    } else if (folders && folders.length > 0) {
      children = folders
        .filter(f => !f.attrs.some(a => a.includes('NoSelect') || a.includes('NonExistent')))
        .map(f => {
          const display = getFolderDisplay(f.name)
          return {
            label: display.label,
            key: `${account.ID}:${f.name}`,
            icon: () => display.icon,
          }
        })
      // If all folders were filtered out, fall back to showing all
      if (children.length === 0) {
        children = folders.map(f => ({
          label: f.name,
          key: `${account.ID}:${f.name}`,
          icon: () => '📁',
        }))
      }
    }

    options.push({
      label: account.Name || account.Email,
      key: account.ID,
      icon: iconSource
        ? () => iconSource.create()
        : () => account.Email.charAt(0).toUpperCase(),
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
      :icon-size="24"
      :collapsed-icon-size="24"
      accordion
      :expanded-keys="expandedKeys"
      :options="menuOptions"
      @update:value="handleUpdateValue"
      @update:expanded-keys="onExpandedKeysChange"
    />
  </NScrollbar>
</template>

<style scoped></style>
