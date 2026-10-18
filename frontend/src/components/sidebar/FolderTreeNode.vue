<script setup lang="ts">
import { computed } from 'vue'
import type { FolderTreeNode } from '~/composables/folder-tree'
import UiBadge from '~/components/ui/UiBadge.vue'

const props = defineProps<{
  node: FolderTreeNode
  accountId: string
  depth: number
  expandedFolders: string[]
  selectedFolderKey: string
}>()

const emit = defineEmits<{
  toggle: [folderName: string]
  select: [folderName: string]
}>()

const key = computed(() => `${props.accountId}\u200b${props.node.fullName}`)
const isExpanded = computed(() => props.expandedFolders.includes(key.value))
const isSelected = computed(() => props.selectedFolderKey === key.value)
const hasChildren = computed(() => props.node.children.length > 0)
</script>

<template>
  <div>
    <div
      class="folder-row"
      :class="{ 'folder-row--selected': isSelected }"
      :style="{ paddingLeft: `${18 + depth * 14}px` }"
    >
      <button
        v-if="hasChildren"
        type="button"
        class="folder-toggle"
        :aria-label="isExpanded ? 'Collapse folder' : 'Expand folder'"
        :aria-expanded="isExpanded"
        @click.stop="emit('toggle', node.fullName)"
      >
        <i
          class="i-material-symbols:chevron-right"
          :class="{ 'folder-toggle--open': isExpanded }"
          aria-hidden="true"
        />
      </button>
      <span v-else class="folder-toggle-spacer" aria-hidden="true" />

      <button
        v-if="node.selectable"
        type="button"
        class="folder-select"
        :aria-current="isSelected ? 'page' : undefined"
        @click="emit('select', node.fullName)"
      >
        <i :class="`i-${node.icon}`" class="f-icon" aria-hidden="true" />
        <span class="folderText">{{ node.label }}</span>
        <UiBadge :count="node.unread ?? 0" />
      </button>
      <span v-else class="folder-label">
        <i :class="`i-${node.icon}`" class="f-icon" aria-hidden="true" />
        <span class="folderText">{{ node.label }}</span>
      </span>
    </div>

    <div v-if="hasChildren && isExpanded" class="folder-children">
      <FolderTreeNode
        v-for="child in node.children"
        :key="child.fullName"
        :node="child"
        :account-id="accountId"
        :depth="depth + 1"
        :expanded-folders="expandedFolders"
        :selected-folder-key="selectedFolderKey"
        @toggle="emit('toggle', $event)"
        @select="emit('select', $event)"
      />
    </div>
  </div>
</template>

<style scoped>
.folder-row {
  display: flex;
  align-items: center;
  gap: 2px;
  min-width: 0;
  min-height: 27px;
  padding-top: 1px;
  padding-right: 4px;
  padding-bottom: 1px;
  border-radius: 4px;
}
.folder-row:hover {
  background: var(--n-sidebar-hover);
}
.folder-row--selected {
  background: var(--n-sidebar-active);
}
.folder-toggle,
.folder-toggle-spacer {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 14px;
  width: 14px;
  height: 22px;
  padding: 0;
  border: 0;
  color: var(--color-text-3);
  background: transparent;
}
.folder-toggle {
  cursor: pointer;
}
.folder-toggle i {
  transition: transform 0.12s ease;
}
.folder-toggle--open {
  transform: rotate(90deg);
}
.folder-select,
.folder-label {
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
  gap: 6px;
  padding: 3px 0;
  border: 0;
  color: var(--n-text-color);
  background: transparent;
  text-align: left;
  white-space: nowrap;
  overflow: hidden;
}
.folder-select {
  cursor: pointer;
}
.folder-select:hover {
  color: var(--n-primary-color);
}
.folder-label {
  opacity: 0.85;
}
.folder-children {
  display: flex;
  flex-direction: column;
  gap: 1px;
}
.f-icon {
  flex: 0 0 16px;
  font-size: 15px;
}
.folderText {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 13px;
}
</style>
