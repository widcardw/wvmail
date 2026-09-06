<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  TreeView,
  createTreeCollection,
  type TreeViewSelectionChangeDetails,
} from '@ark-ui/vue/tree-view'
import { useI18n } from '../i18n'
import { useAccountsStore } from '../stores/accounts'
import { compareFolders, getFolderDisplay } from '../types/email'
import type { EmailAccount } from '../types/email'
import styles from './SideBar.module.css'
import AccountAvatar from './AccountAvatar.vue'

const { t, locale } = useI18n()
const store = useAccountsStore()
const router = useRouter()

interface SideNode {
  id: string
  name: string
  kind: 'account' | 'folder'
  iconClass?: string
  childrenCount?: number
  children?: SideNode[]
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

/** Build an account node: folders become its children only once cached in the store. */
function accountNode(account: EmailAccount): SideNode {
  const cached = store.foldersByAccount[account.ID]
  const children: SideNode[] = []
  if (cached) {
    const list =
      cached.filter(
        (f) => !f.attrs.some((a) => a.includes('NoSelect') || a.includes('NonExistent')),
      ) ?? []
    const sorted = [...list].sort((a, b) => compareFolders(a, b, locale.value))
    for (const f of sorted) {
      const d = getFolderDisplay(f.name, locale.value)
      children.push({
        id: fkey(account.ID, f.name),
        name: d.label,
        kind: 'folder',
        iconClass: `i-${d.icon}`,
      })
    }
  }
  const node: SideNode = {
    id: account.ID,
    name: account.Name || account.Email,
    kind: 'account',
    children,
    // childrenCount keeps it an expandable branch (Ark) even when folders not loaded yet
    childrenCount: cached ? children.length : 0,
  }
  return node
}

const collection = ref<ReturnType<typeof createTreeCollection<SideNode>> | null>(null)

function rebuild() {
  collection.value = createTreeCollection<SideNode>({
    nodeToValue: (n) => n.id,
    nodeToString: (n) => n.name,
    isNodeDisabled: (n) => n.kind !== 'folder',
    rootNode: { id: 'ROOT', name: '', kind: 'folder', children: store.accounts.map(accountNode) },
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
// Rebuild when folder cache changes (folder list just loaded), which adds the leaf nodes.
watch(
  () =>
    store.accounts
      .map((a) => (store.foldersByAccount[a.ID] ? store.foldersByAccount[a.ID].length : -1))
      .join(','),
  () => rebuild(),
)

const expanded = ref<string[]>([])
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

function onSelectionChange(details: TreeViewSelectionChangeDetails) {
  const v = details.selectedValue?.[0]
  if (!v) return
  const sep = v.indexOf('\u200b')
  if (sep < 0) return
  const accountId = v.slice(0, sep)
  const folderName = v.slice(sep + 1)
  store.selectFolder(accountId, folderName)
  router.push('/')
}

async function onSmart(key: string) {
  // Smart folders aren't part of the account TreeView — collapse & clear the tree
  // first so an earlier selection/expansion state doesn't linger visually.
  selected.value = []
  expanded.value = []
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

function syncSelected() {
  if (!store.isSmartFolder && store.selectedAccountId && store.selectedFolder) {
    selected.value = [fkey(store.selectedAccountId, store.selectedFolder)]
  } else {
    selected.value = []
  }
}
watch(() => [store.selectedAccountId, store.selectedFolder, store.smartFolder], syncSelected)
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
        selection-mode="single"
        expand-on-click
        v-model:selected-value="selected"
        v-model:expanded-value="expanded"
        @selection-change="onSelectionChange"
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
                </TreeView.BranchControl>

                <TreeView.BranchContent v-if="isOpen(account.ID)" :class="styles.folders">
                  <template
                    v-for="(folder, fi) in accountNode(account).children ?? []"
                    :key="folder.id"
                  >
                    <TreeView.NodeProvider :node="folder" :indexPath="[ai, fi]">
                      <TreeView.Item :class="styles.folder">
                        <i :class="folder.iconClass" class="f-icon" aria-hidden="true" />
                        <TreeView.ItemText :class="styles.folderText">{{
                          folder.name
                        }}</TreeView.ItemText>
                      </TreeView.Item>
                    </TreeView.NodeProvider>
                  </template>
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
