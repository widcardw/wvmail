<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { TreeView, createTreeCollection } from '@ark-ui/vue/tree-view'
import { useI18n } from '~/i18n'
import { useAccountsStore } from '~/stores/accounts'
import { buildFolderTree } from '~/composables/folder-tree'
import { getFolderDisplay } from '~/types/email'
import type { FolderTreeNode } from '~/composables/folder-tree'
import type { EmailAccount } from '~/types/email'
import styles from './SideBar.module.css'
import AccountAvatar from '~/components/icon/AccountAvatar.vue'
import FolderTreeNodeView from './FolderTreeNode.vue'
import UiBadge from '~/components/ui/UiBadge.vue'

const { t, tf, locale } = useI18n()
const store = useAccountsStore()
const router = useRouter()

interface SideNode extends FolderTreeNode {
  id: string
  name: string
  kind: 'account' | 'folder'
  childrenCount?: number
  children: SideNode[]
}

const smartRows = [
  { key: 'inbox', labelKey: 'sidebar.all_inbox', icon: 'ri:inbox-archive-line' },
  { key: 'sent', labelKey: 'sidebar.all_sent', icon: 'ri:send-plane-2-line' },
  { key: 'drafts', labelKey: 'sidebar.all_drafts', icon: 'ri:draft-line' },
] as const

const utilRows = [
  {
    key: 'reminder',
    labelKey: 'reminders.title',
    icon: 'ri:notification-2-line',
    path: '/reminder',
  },
  { key: 'notes', labelKey: 'notes.title', icon: 'ri:pen-nib-line', path: '/notes' },
  { key: 'settings', labelKey: 'settings', icon: 'ri:settings-2-line', path: '/settings' },
] as const

function fkey(id: string, f: string) {
  return `${id}\u200b${f}`
}

function folderSideNode(accountId: string, folder: FolderTreeNode): SideNode {
  return {
    ...folder,
    id: fkey(accountId, folder.fullName),
    name: folder.label,
    kind: 'folder',
    // Read the store at render time so unread changes update badges without
    // rebuilding the folder hierarchy.
    unread: store.folderUnread(accountId, folder.fullName),
    children: folder.children.map((child) => folderSideNode(accountId, child)),
  }
}

/** Build an account node: folders become its children only once cached in the store. */
function accountNode(account: EmailAccount): SideNode {
  const cached = store.foldersByAccount[account.ID]
  const children = cached
    ? buildFolderTree(cached, (segment) => getFolderDisplay(segment, locale.value)).map((folder) =>
        folderSideNode(account.ID, folder),
      )
    : []
  return {
    id: account.ID,
    fullName: '',
    label: account.Name || account.Email,
    name: account.Name || account.Email,
    icon: '',
    rank: 0,
    selectable: false,
    unread: 0,
    kind: 'account',
    children,
    // childrenCount keeps it an expandable branch (Ark) even when folders not loaded yet.
    childrenCount: cached ? children.length : 0,
  }
}

const collection = ref<ReturnType<typeof createTreeCollection<SideNode>> | null>(null)

function rebuild() {
  collection.value = createTreeCollection<SideNode>({
    nodeToValue: (n) => n.id,
    nodeToString: (n) => n.name,
    isNodeDisabled: (n) => n.kind !== 'folder',
    rootNode: {
      id: 'ROOT',
      name: '',
      fullName: '',
      label: '',
      icon: '',
      rank: 0,
      selectable: false,
      unread: 0,
      kind: 'folder',
      children: store.accounts.map(accountNode),
    },
  })
}
// Ark's same-name types resolve differently across pkg boundaries in some setups;
// route the Root prop through an untyped accessor so IDE type chasing stops here.
function treeCol(): any {
  return collection.value
}

watch(
  () => store.accounts.map((a) => a.ID),
  () => rebuild(),
)
// Rebuild when folder cache changes (folder list loaded/refreshed), updating the hierarchy.
watch(
  () => store.accounts.map((a) => JSON.stringify(store.foldersByAccount[a.ID] ?? null)).join('|'),
  () => rebuild(),
)

const expanded = ref<string[]>([])
const expandedFolders = ref<string[]>([])
const selected = ref<string[]>([])
const inflight = new Set<string>()

onMounted(async () => {
  if (store.accounts.length === 0) await store.loadAccounts()
  rebuild()
  syncSelected()
})

// Lazy-load folders the first time an account branch gets opened.
watch(expanded, (vals) => {
  for (const id of vals) {
    const acc = store.accounts.find((a) => a.ID === id)
    if (acc && !store.foldersByAccount[id] && !inflight.has(id)) {
      inflight.add(id)
      store
        .loadFolders(acc)
        .catch(() => {})
        .finally(() => inflight.delete(id))
    }
  }
})

function toggleFolder(accountId: string, folderName: string) {
  const key = fkey(accountId, folderName)
  expandedFolders.value = expandedFolders.value.includes(key)
    ? expandedFolders.value.filter((value) => value !== key)
    : [...expandedFolders.value, key]
}

function selectFolder(accountId: string, folderName: string) {
  store.selectFolder(accountId, folderName)
  router.push('/')
}

async function onSmart(key: string) {
  // Smart folders aren't part of the account TreeView — collapse & clear the tree
  // first so an earlier selection/expansion state doesn't linger visually.
  selected.value = []
  expanded.value = []
  expandedFolders.value = []
  await store.selectSmartFolder(key)
  router.push('/')
}

function activePath(path: string): boolean {
  const cur = router.currentRoute.value.path
  return path === '/settings' ? cur.startsWith('/settings') : cur === path
}

function smartPathActive(key: string): boolean {
  return router.currentRoute.value.path === '/' && store.smartFolder === key
}

function goNav(path: string) {
  router.push(path)
}

function isOpen(id: string) {
  return expanded.value.includes(id)
}

function onMailView() {
  return router.currentRoute.value.path === '/'
}

function syncSelected() {
  // Only mirror the store into the tree while the mail list is the visible
  // view. On settings/notes/reminder the folder row must lose its highlight:
  // otherwise it still looks selected, and re-clicking it emits no
  // selection-change (the value is unchanged), so the click never navigates
  // back to the folder. The nested folder button remains a normal button, so
  // clicking it can always re-select the current mailbox.
  if (onMailView() && !store.isSmartFolder && store.selectedAccountId && store.selectedFolder) {
    selected.value = [fkey(store.selectedAccountId, store.selectedFolder)]
  } else {
    selected.value = []
  }
}
watch(() => [store.selectedAccountId, store.selectedFolder, store.smartFolder], syncSelected)
// Leaving the mail view clears the folder highlight; coming back re-applies it
// from the store (syncSelected), so the selection survives the round trip.
watch(() => router.currentRoute.value.path, syncSelected)
</script>

<template>
  <div class="side space-y-1px">
    <button
      v-for="row in smartRows"
      :key="row.key"
      type="button"
      class="row"
      :class="{ 'row--active': smartPathActive(row.key) }"
      @click="onSmart(row.key)"
    >
      <i :class="`i-${row.icon}`" class="row-icon" aria-hidden="true" />
      <span>{{ t[row.labelKey] }}</span>
    </button>

    <div class="divider" />

    <div class="tree-wrap">
      <p v-if="store.accounts.length === 0" class="empty">{{ t['settings.accounts.empty'] }}</p>

      <TreeView.Root
        v-else-if="collection"
        :collection="treeCol()"
        v-model:expanded-value="expanded"
      >
        <TreeView.Tree :class="styles.tree">
          <template v-for="(account, ai) in store.accounts" :key="account.ID">
            <TreeView.NodeProvider :node="accountNode(account)" :indexPath="[ai]">
              <TreeView.Branch>
                <TreeView.BranchControl :class="styles.ctrl">
                  <TreeView.BranchIndicator :class="styles.indicator">
                    <i class="i-material-symbols:chevron-right" aria-hidden="true" />
                  </TreeView.BranchIndicator>
                  <TreeView.BranchText :class="styles.branchText">
                    <AccountAvatar :account-id="account.ID" :size="18" />
                    <span class="acc-name">{{ account.Name || account.Email }}</span>
                  </TreeView.BranchText>
                  <!-- Account badge: total unread across the account's folders (the
                       same server-side counts the folder badges read). Shows as soon
                       as the startup per-account fetch lands, no expand needed. -->
                  <UiBadge
                    class="acc-badge"
                    :count="store.accountUnread(account.ID)"
                    :label="tf('sidebar.unread', { count: store.accountUnread(account.ID) })"
                  />
                </TreeView.BranchControl>

                <TreeView.BranchContent v-if="isOpen(account.ID)" :class="styles.folders">
                  <FolderTreeNodeView
                    v-for="folder in accountNode(account).children"
                    :key="folder.id"
                    :node="folder"
                    :account-id="account.ID"
                    :depth="0"
                    :expanded-folders="expandedFolders"
                    :selected-folder-key="selected[0] ?? ''"
                    @toggle="toggleFolder(account.ID, $event)"
                    @select="selectFolder(account.ID, $event)"
                  />
                </TreeView.BranchContent>
              </TreeView.Branch>
            </TreeView.NodeProvider>
          </template>
        </TreeView.Tree>
      </TreeView.Root>
    </div>

    <div class="divider" />

    <div class="util space-y-1px">
      <button
        v-for="row in utilRows"
        :key="row.key"
        type="button"
        class="row"
        :class="{ 'row--active': activePath(row.path) }"
        @click="goNav(row.path)"
      >
        <i :class="`i-${row.icon}`" class="row-icon" aria-hidden="true" />
        <span>{{ t[row.labelKey] }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.side {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  padding: 6px;
}

.row {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  margin: 0;
  padding: 6px 10px;
  border: none;
  background: transparent;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  text-align: left;
  white-space: nowrap;
  overflow: hidden;
  color: var(--n-text-color);
}
.row:hover {
  background: var(--n-sidebar-hover);
}
.row-icon {
  flex: 0 0 16px;
  font-size: 16px;
}

.divider {
  height: 1px;
  margin: 6px 8px;
  background: var(--n-border-color);
  opacity: 0.7;
}

.tree-wrap {
  flex: 1;
  min-height: 0;
  overflow: auto;
}

.empty {
  padding: 12px;
  font-size: 12px;
  opacity: 0.6;
  color: var(--n-text-color);
}

.acc-name {
  overflow: hidden;
  text-overflow: ellipsis;
}
.acc-badge {
  margin-left: auto;
  flex: none;
  margin-right: 6px;
}
.f-icon {
  flex: 0 0 16px;
  font-size: 15px;
}

.util {
  flex-shrink: 0;
}
.row--active {
  background: var(--n-sidebar-active);
  color: var(--n-text-color);
}
</style>
