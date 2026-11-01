<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Menu } from '@ark-ui/vue/menu'
import { useI18n } from '~/i18n'
import { useNotesStore } from '~/stores/notes'
import { useFeedbackStore } from '~/stores/feedback'
import { formatNoteDate, noteHtmlToText, notePreviewText } from '~/composables/note-text'
import NoteEditor from '~/components/notes/NoteEditor.vue'
import UiScrollArea from '~/components/ui/UiScrollArea.vue'
import ConfirmDialog from '~/components/ui/ConfirmDialog.vue'
import type { Note } from '~/stores/notes'

// The list half of Notes. Everything about editing a single note (editor instance,
// toolbar, autosave) lives in components/notes/NoteEditor.vue, which this view mounts
// with `:key="note.id"` so each note gets its own editor — and its own undo history.
const { t, tf } = useI18n()
const notesStore = useNotesStore()
const feedback = useFeedbackStore()

const filter = ref('')
const openMenuId = ref<string | null>(null)
const deleteTarget = ref<Note | null>(null)
const deleting = ref(false)
const editorRef = ref<{
  flush: () => Promise<void>
  discard: () => void
  restore: () => void
} | null>(null)

/** Client-side filter over the loaded notes (the backend has no note search). */
const visibleNotes = computed(() => {
  const needle = filter.value.trim().toLowerCase()
  if (!needle) return notesStore.sortedNotes
  return notesStore.sortedNotes.filter((note) => {
    const text = `${note.title} ${noteHtmlToText(note.content, ' ')}`.toLowerCase()
    return text.includes(needle)
  })
})

async function handleNewNote() {
  await notesStore.createNote()
}

function onMenuOpenChange(note: Note, open: boolean) {
  if (open) {
    openMenuId.value = note.id
  } else if (openMenuId.value === note.id) {
    openMenuId.value = null
  }
}

function onMenuSelect(note: Note, value: string | undefined) {
  if (value !== 'delete') return
  // The item's own click handler also calls this; keep the two paths idempotent.
  if (deleteTarget.value?.id === note.id) return
  // The dialog is modal; dismiss the popup before its backdrop appears.
  openMenuId.value = null
  deleteTarget.value = note
}

async function confirmDelete() {
  const target = deleteTarget.value
  if (!target || deleting.value) return

  deleting.value = true
  // The editor may be holding an unsaved edit for this note; discarding it stops the
  // unmount flush from writing the note back right after it was deleted.
  if (notesStore.selectedId === target.id) editorRef.value?.discard()
  const deleted = await notesStore.deleteNote(target.id)
  deleting.value = false

  if (!deleted) {
    // A failed delete leaves the note open, so restore its pending autosave.
    if (notesStore.selectedId === target.id) editorRef.value?.restore()
    feedback.error(t.value['notes.deleteFailed'])
    return
  }

  deleteTarget.value = null
  feedback.success(t.value['notes.deleted'])
}

function switchNote(id: string) {
  notesStore.selectNote(id)
}

onMounted(() => {
  void notesStore.loadNotes()
})
</script>
<template>
  <div class="notes-view">
    <!-- Sidebar -->
    <div class="notes-sidebar">
      <div class="sidebar-header">
        <span class="sidebar-title">{{ t['notes.title'] }}</span>
        <button type="button" class="btn btn--primary" @click="handleNewNote">
          <i class="i-ri:pen-nib-fill" aria-hidden="true" />
          {{ t['notes.new'] }}
        </button>
      </div>
      <div class="sidebar-filter">
        <i class="i-ri:search-line filter-icon" aria-hidden="true" />
        <input
          v-model="filter"
          class="note-filter"
          type="search"
          spellcheck="false"
          :placeholder="t['notes.search']"
          :aria-label="t['notes.search']"
        />
        <button
          v-if="filter"
          type="button"
          class="filter-clear"
          :aria-label="t['notes.clearSearch']"
          @click="filter = ''"
        >
          <i class="i-ri:close-line" aria-hidden="true" />
        </button>
      </div>
      <UiScrollArea class="scroll">
        <p v-if="visibleNotes.length === 0" class="empty-pad">
          {{ filter.trim() ? t['notes.noMatch'] : t['notes.empty'] }}
        </p>
        <Menu.Root
          v-for="note in visibleNotes"
          :key="note.id"
          :open="openMenuId === note.id"
          :positioning="{ placement: 'bottom-start', gutter: 4 }"
          lazy-mount
          unmount-on-exit
          @open-change="
            (details: unknown) => onMenuOpenChange(note, !!(details as { open?: boolean })?.open)
          "
          @select="
            (details: unknown) =>
              onMenuSelect(note, (details as { value?: string })?.value ?? undefined)
          "
        >
          <Menu.ContextTrigger as-child>
            <button
              type="button"
              class="note-item"
              :class="{ selected: note.id === notesStore.selectedId }"
              @click="switchNote(note.id)"
            >
              <span class="note-item-content">
                <span class="note-title">
                  {{ notePreviewText(note.content, t['notes.untitled']) }}
                </span>
                <span class="note-date">{{ formatNoteDate(note.updated_at) }}</span>
              </span>
            </button>
          </Menu.ContextTrigger>

          <Teleport to="body">
            <Menu.Positioner class="note-menu-pos">
              <Menu.Content class="note-menu">
                <Menu.Item
                  value="delete"
                  class="note-menu__item note-menu__item--danger"
                  @click="onMenuSelect(note, 'delete')"
                >
                  <i class="i-ri:delete-bin-5-line" aria-hidden="true" />
                  <Menu.ItemText>{{ t['notes.delete'] }}</Menu.ItemText>
                </Menu.Item>
              </Menu.Content>
            </Menu.Positioner>
          </Teleport>
        </Menu.Root>
      </UiScrollArea>
    </div>

    <!-- Editor: one instance per note (see NoteEditor.vue for why the key matters) -->
    <NoteEditor
      v-if="notesStore.selectedNote"
      :key="notesStore.selectedNote.id"
      ref="editorRef"
      :note="notesStore.selectedNote"
    />

    <div v-else class="editor-placeholder">
      <p class="empty-pad">{{ t['notes.empty'] }}</p>
    </div>

    <ConfirmDialog
      :open="!!deleteTarget"
      :title="t['notes.delete']"
      :description="
        deleteTarget
          ? tf('notes.deleteConfirm', {
              title: notePreviewText(deleteTarget.content, t['notes.untitled']),
            })
          : undefined
      "
      :confirm-label="t['notes.delete']"
      danger
      :busy="deleting"
      @update:open="(open) => (!open ? (deleteTarget = null) : undefined)"
      @confirm="confirmDelete"
    />
  </div>
</template>

<style scoped>
.notes-view {
  height: 100%;
  display: flex;
  flex-direction: row;
  color: var(--n-text-color);
}

/* A bare row, full width, closed by a divider: no box, no radius — the sidebar itself
   provides the frame (`.input`'s recipe and `type="search"`'s native chrome both made
   it look like a form control dropped into the list). The wrapper owns the row and the
   input is transparent inside it. */
.sidebar-filter {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  height: 32px;
  padding: 0 12px;
  color: var(--color-text-3);
  border-bottom: 1px solid var(--n-border-color);
}
.sidebar-filter:focus-within {
  color: var(--n-text-color);
  /* Thickens the existing divider instead of adding/removing a border, so focusing
     the field cannot shift the list by a pixel. */
  box-shadow: inset 0 -1px 0 var(--n-primary-color);
}
.filter-icon {
  flex-shrink: 0;
  font-size: 14px;
}
.note-filter {
  flex: 1;
  min-width: 0;
  height: 100%;
  padding: 0;
  font-size: 12.5px;
  color: inherit;
  background: transparent;
  border: none;
  outline: none;
  appearance: none;
}
/* Without this WebKit keeps the native rounded field look and its inline clear
   button, which ignores our box (and shifts the placeholder). */
.note-filter::-webkit-search-cancel-button,
.note-filter::-webkit-search-decoration {
  appearance: none;
}
.note-filter::placeholder {
  color: var(--color-text-3);
}
.filter-clear {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  padding: 0;
  color: inherit;
  background: none;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}
.filter-clear:hover {
  color: var(--n-text-color);
  background: var(--n-sidebar-hover);
}
.filter-clear i {
  font-size: 14px;
}
.notes-sidebar {
  width: 260px;
  height: 100%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--n-border-color);
  flex-shrink: 0;
}

.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  height: var(--pane-header-height);
  padding: 0 12px;
  border-bottom: 1px solid var(--n-border-color);
  flex-shrink: 0;
}
.sidebar-title {
  font-size: 14px;
  font-weight: 600;
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  font-size: 12px;
  border: none;
  border-radius: 5px;
  cursor: pointer;
}
.btn--primary {
  color: #fff;
  background: var(--n-primary-color);
}
.btn--primary:hover {
  background: var(--n-primary-hover);
}

.scroll {
  flex: 1;
  min-height: 0;
}

.note-item {
  display: flex;
  align-items: center;
  width: 100%;
  border: none;
  /* Divider between rows (the row is a <button>, so `border: none` above has to come
     first). */
  border-bottom: 1px solid var(--n-border-color);
  border-left: 2px solid transparent;
  background: transparent;
  padding: 0;
  text-align: left;
  cursor: pointer;
  color: inherit;
}
.note-item:hover {
  background: var(--n-sidebar-hover);
}
.note-item.selected {
  border-left-color: var(--n-primary-color);
  background: var(--n-sidebar-active);
}
.note-item-content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  padding: 6px 8px;
}
.note-title {
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: inherit;
}
.note-date {
  font-size: 11px;
  opacity: 0.5;
}

/* Portalled to <body>, so the scoped attribute is carried by these nodes themselves. */
.note-menu-pos {
  z-index: 1000;
}
.note-menu {
  min-width: 152px;
  padding: 4px;
  color: var(--n-text-color);
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
  box-shadow: 0 8px 20px rgb(0 0 0 / 18%);
}
.note-menu__item {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr);
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
.note-menu__item[data-highlighted] {
  background: var(--n-sidebar-hover);
}
.note-menu__item--danger {
  color: var(--color-danger);
}
.note-menu__item--danger[data-highlighted] {
  background: var(--color-danger-bg);
}

.editor-placeholder {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}
.empty-pad {
  margin: 0;
  padding: 24px 16px;
  color: var(--color-text-3);
  font-size: 13px;
}
</style>
