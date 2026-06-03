<script setup lang="ts">
import { NCollapseItem, NAvatar } from 'naive-ui'

export interface EmailAccount {
  id: string
  name: string
  email: string
  color: string
}

export interface Folder {
  id: string
  label: string
  icon: string
}

defineProps<{
  account: EmailAccount
  folders: Folder[]
  active?: boolean
}>()

const emit = defineEmits<{
  selectFolder: [accountId: string, folderId: string]
}>()
</script>

<template>
  <NCollapseItem :name="account.id" class="account-item">
    <template #header>
      <div class="account-header">
        <NAvatar :color="account.color" round size="small" style="flexshrink: 0">
          {{ account.name.charAt(0).toUpperCase() }}
        </NAvatar>
        <span class="account-name">{{ account.name }}</span>
      </div>
    </template>
    <div class="folder-list">
      <div
        v-for="folder in folders"
        :key="folder.id"
        class="folder-item"
        @click="emit('selectFolder', account.id, folder.id)"
      >
        <span class="folder-icon">{{ folder.icon }}</span>
        <span class="folder-label">{{ folder.label }}</span>
      </div>
    </div>
  </NCollapseItem>
</template>

<style scoped>
.account-header {
  display: flex;
  align-items: center;
  gap: 8px;
  /*padding: 2px 0;*/
}

/*.account-name {
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}*/

/*.account-item :deep(.n-collapse-item__header) {
  padding: 6px 8px;
  border-radius: 6px;
}*/

/*.account-item :deep(.n-collapse-item__header:hover) {
  background-color: var(--n-sidebar-hover);
}*/

/*.account-item.active :deep(.n-collapse-item__header) {
  background-color: var(--n-sidebar-active);
}*/

/*.account-item :deep(.n-collapse-item__content-wrapper) {
  overflow: visible;
}*/

/*.account-item :deep(.n-collapse-item__content-inner) {
  padding: 0;
}*/

.folder-list {
  padding: 4px 0 4px 48px;
}

.folder-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  cursor: pointer;
  border-radius: 4px;
  font-size: 13px;
  transition: background-color 0.15s;
}

.folder-item:hover {
  background-color: var(--n-sidebar-hover);
}

.folder-icon {
  font-size: 14px;
  width: 20px;
  text-align: center;
}

.folder-label {
  opacity: 0.8;
}
</style>
