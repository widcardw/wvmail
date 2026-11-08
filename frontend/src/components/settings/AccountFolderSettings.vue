<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from '~/i18n'
import type { AccountFolderSetting, FolderInfo } from '#/wvmail/services'
import { getFolderDisplay } from '~/composables/folder-semantics'
import { buildFolderTree, type FolderTreeNode } from '~/composables/folder-tree'
import { folderSettingFor, orderFolderNodes } from '~/composables/folder-settings'
import FolderIconSelect from '~/components/settings/FolderIconSelect.vue'
import UiSwitch from '~/components/ui/UiSwitch.vue'

const props = defineProps<{
  folders: FolderInfo[]
  loading: boolean
  error: string
  modelValue: AccountFolderSetting[]
  order?: string[]
}>()

const emit = defineEmits<{
  'update:modelValue': [settings: AccountFolderSetting[]]
  'update:order': [order: string[]]
  refresh: []
}>()

const { t, locale } = useI18n()

function settingFor(folderName: string): AccountFolderSetting {
  const found = folderSettingFor(props.modelValue, folderName)
  return {
    Name: folderName,
    Hidden: found?.Hidden === true,
    DisplayName: found?.DisplayName ?? '',
    Icon: found?.Icon ?? '',
  }
}

function updateSetting(folderName: string, patch: Partial<AccountFolderSetting>) {
  const current = settingFor(folderName)
  const hidden = patch.Hidden ?? current.Hidden
  const displayName = patch.DisplayName ?? current.DisplayName
  const icon = patch.Icon ?? current.Icon
  const next = props.modelValue.filter((setting) => setting.Name !== folderName)

  // Keep config.yaml clean: a folder that is visible with no custom name or icon
  // is exactly the default state, so it needs no stored entry.
  if (hidden || displayName.trim() || icon.trim()) {
    next.push({
      Name: folderName,
      Hidden: hidden,
      DisplayName: displayName,
      Icon: icon,
    })
  }
  emit('update:modelValue', next)
}

type FolderRow = {
  /** Full IMAP name: the key shown in the row and what settings are stored under. */
  name: string
  depth: number
  /** Full name of the parent node, or '' for a top-level folder. */
  parent: string
  automatic: { label: string; icon: string; rank: number }
  setting: AccountFolderSetting
  hidden: boolean
  label: string
}

/**
 * The settings list mirrors the sidebar's tree, flattened in display order:
 * this is what makes the default order match the sidebar without the user
 * dragging every row into place. Hidden folders stay in the list (they need a
 * switch to come back), so only the order — not `applyFolderSettings` — is
 * applied here.
 */
const rows = computed<FolderRow[]>(() => {
  const tree = buildFolderTree(
    props.folders.map((folder) => ({
      name: folder.name,
      delim: folder.delim,
      attrs: folder.attrs ?? [],
    })),
    (segment, attrs) => getFolderDisplay(segment, locale.value, attrs),
  )

  const flat: FolderRow[] = []
  const walk = (nodes: FolderTreeNode[], depth: number, parent: string) => {
    for (const node of nodes) {
      const setting = settingFor(node.fullName)
      flat.push({
        name: node.fullName,
        depth,
        parent,
        automatic: { label: node.label, icon: node.icon, rank: node.rank },
        setting,
        hidden: setting.Hidden === true,
        label: setting.DisplayName?.trim() || node.label,
      })
      walk(node.children, depth + 1, node.fullName)
    }
  }
  walk(orderFolderNodes(tree, props.order), 0, '')
  return flat
})

/**
 * The name field is auto-filled with the sidebar's effective name. While it is
 * being edited the raw draft wins, so clearing the field does not immediately
 * snap back to the detected name mid-typing; blurring drops the draft and shows
 * the resolved value again.
 */
const nameDrafts = ref<Record<string, string>>({})

function nameValue(row: FolderRow): string {
  return nameDrafts.value[row.name] ?? row.label
}

function onNameInput(row: FolderRow, event: Event) {
  const value = (event.target as HTMLInputElement).value
  nameDrafts.value = { ...nameDrafts.value, [row.name]: value }
  // Typing the detected name back is the default state, so store nothing.
  updateSetting(row.name, {
    DisplayName: value.trim() === row.automatic.label ? '' : value,
  })
}

function clearNameDraft(folderName: string) {
  if (!(folderName in nameDrafts.value)) return
  const next = { ...nameDrafts.value }
  delete next[folderName]
  nameDrafts.value = next
}

const draggingName = ref<string | null>(null)
const dropTargetName = ref<string | null>(null)

function rowByName(folderName: string): FolderRow | undefined {
  return rows.value.find((row) => row.name === folderName)
}

function onDragStart(folderName: string, event: DragEvent) {
  draggingName.value = folderName
  event.dataTransfer?.setData('text/plain', folderName)
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
}

function onDragOver(row: FolderRow, event: DragEvent) {
  const source = draggingName.value === null ? undefined : rowByName(draggingName.value)
  if (!source || source.name === row.name) return
  // Ordering only means something among siblings (see orderFolderNodes); refuse
  // a drop that would try to move a folder out of its parent.
  if (source.parent !== row.parent) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
  dropTargetName.value = row.name
}

function onDrop(row: FolderRow, event: DragEvent) {
  event.preventDefault()
  const source = draggingName.value
  draggingName.value = null
  dropTargetName.value = null
  if (!source || source === row.name) return
  const sourceRow = rowByName(source)
  if (!sourceRow || sourceRow.parent !== row.parent) return

  const next = rows.value.map((candidate) => candidate.name)
  const from = next.indexOf(source)
  const to = next.indexOf(row.name)
  if (from === -1 || to === -1) return
  const [moved] = next.splice(from, 1)
  next.splice(to, 0, moved)
  emit('update:order', next)
}

function onDragEnd() {
  draggingName.value = null
  dropTargetName.value = null
}

/** Ignore dragleave fired while moving between the row's own children. */
function onDragLeave(folderName: string, event: DragEvent) {
  const related = event.relatedTarget as Node | null
  if (related && (event.currentTarget as HTMLElement).contains(related)) return
  if (dropTargetName.value === folderName) dropTargetName.value = null
}

/** Indent nested mailboxes so the list reads like the sidebar tree. */
function rowIndent(row: FolderRow): string {
  return `${8 + Math.min(row.depth, 6) * 18}px`
}

function setVisible(folderName: string, visible: boolean) {
  updateSetting(folderName, { Hidden: !visible })
}

function setIcon(folderName: string, icon: string) {
  updateSetting(folderName, { Icon: icon })
}
</script>

<template>
  <div class="folder-settings">
    <header class="folder-settings__head">
      <div>
        <h3>{{ t('settings.accounts.folders.title') }}</h3>
        <p>{{ t('settings.accounts.folders.hint') }}</p>
      </div>
      <button
        type="button"
        class="icon-btn"
        :title="t('settings.accounts.folders.refresh')"
        :aria-label="t('settings.accounts.folders.refresh')"
        :disabled="props.loading"
        @click="emit('refresh')"
      >
        <i
          class="i-ri:refresh-line"
          :class="{ 'folder-settings__spin': props.loading }"
          aria-hidden="true"
        />
      </button>
    </header>

    <div v-if="props.error" class="folder-settings__state folder-settings__state--error">
      <i class="i-ri:error-warning-line" aria-hidden="true" />
      <span>{{ props.error }}</span>
    </div>
    <div v-else-if="props.loading && props.folders.length === 0" class="folder-settings__state">
      <span class="folder-settings__spinner" aria-hidden="true" />
      <span>{{ t('settings.accounts.folders.loading') }}</span>
    </div>
    <div v-else-if="props.folders.length === 0" class="folder-settings__state">
      <span>{{ t('settings.accounts.folders.empty') }}</span>
    </div>

    <div v-else class="folder-settings__list">
      <div
        v-for="row in rows"
        :key="row.name"
        class="folder-row"
        :class="{
          'folder-row--hidden': row.hidden,
          'folder-row--dragging': draggingName === row.name,
          'folder-row--drop-target': dropTargetName === row.name,
        }"
        :style="{ paddingLeft: rowIndent(row) }"
        :data-depth="row.depth"
        @dragover="onDragOver(row, $event)"
        @drop="onDrop(row, $event)"
        @dragleave="onDragLeave(row.name, $event)"
      >
        <span
          class="folder-row__handle"
          draggable="true"
          :title="t('settings.accounts.folders.drag')"
          @dragstart="onDragStart(row.name, $event)"
          @dragend="onDragEnd"
        >
          <i class="i-ri:drag-move-2-line" aria-hidden="true" />
        </span>
        <span class="folder-row__key" :title="row.name">{{ row.name }}</span>
        <UiSwitch
          :checked="!row.hidden"
          size="sm"
          :aria-label="
            row.hidden
              ? t('settings.accounts.folders.hidden')
              : t('settings.accounts.folders.visible')
          "
          @update:checked="setVisible(row.name, $event)"
        />
        <input
          class="input folder-row__name"
          type="text"
          :value="nameValue(row)"
          :disabled="row.hidden"
          :placeholder="row.automatic.label"
          :title="t('settings.accounts.folders.custom_preview', { name: row.label })"
          :aria-label="t('settings.accounts.folders.display_name')"
          @input="onNameInput(row, $event)"
          @blur="clearNameDraft(row.name)"
        />
        <FolderIconSelect
          :model-value="row.setting.Icon ?? ''"
          :auto-icon="row.automatic.icon"
          :disabled="row.hidden"
          width="160px"
          @update:model-value="setIcon(row.name, $event)"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.folder-settings {
  padding: 12px 0;
}
.folder-settings__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 0 0 12px;
}
.folder-settings__head h3 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}
.folder-settings__head p {
  max-width: 640px;
  margin: 4px 0 0;
  color: var(--color-text-3);
  font-size: 12px;
  line-height: 1.5;
}
.folder-settings__state {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 20px 12px;
  color: var(--color-text-3);
  font-size: 13px;
  border: 1px dashed var(--n-border-color);
  border-radius: 6px;
}
.folder-settings__state--error {
  color: var(--color-danger);
}
.folder-settings__list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}
/* One row per folder: handle · key · switch · display name · icon. The row keeps
   its full height when hidden; the name and icon controls stay in place but are
   disabled, so toggling a folder never makes the list jump. */
.folder-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  padding: 5px 8px;
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  background: var(--color-surface);
}
.folder-row--hidden {
  background: color-mix(in srgb, var(--color-surface) 82%, var(--n-action-color));
}
.folder-row--dragging {
  opacity: 0.55;
}
.folder-row--drop-target {
  border-color: var(--n-primary-color);
}
.folder-row__handle {
  display: inline-flex;
  flex: 0 0 18px;
  align-items: center;
  justify-content: center;
  color: var(--color-text-3);
  font-size: 16px;
  cursor: grab;
}
.folder-row__handle:active {
  cursor: grabbing;
}
.folder-row__handle[draggable='true'] {
  touch-action: none;
}
.folder-row__key {
  flex: 0 0 12rem;
  min-width: 6rem;
  overflow: hidden;
  color: var(--n-text-color);
  font-size: 13px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.folder-row--hidden .folder-row__key {
  color: var(--color-text-3);
}
.folder-row__name {
  flex: 1 1 10rem;
  min-width: 8rem;
}
.folder-settings__spin {
  animation: folder-settings-spin 0.8s linear infinite;
}
.folder-settings__spinner {
  width: 14px;
  height: 14px;
  border: 2px solid var(--n-border-color);
  border-top-color: var(--n-primary-color);
  border-radius: 50%;
  animation: folder-settings-spin 0.8s linear infinite;
}
@keyframes folder-settings-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
