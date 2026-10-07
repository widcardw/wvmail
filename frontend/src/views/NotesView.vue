<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from '~/i18n'
import { useNotesStore } from '~/stores/notes'
import { formatNoteDate, noteHtmlToText, notePreviewText } from '~/composables/note-text'
import NoteEditor from '~/components/notes/NoteEditor.vue'

// The list half of Notes. Everything about editing a single note (editor instance,
// toolbar, autosave) lives in components/notes/NoteEditor.vue, which this view mounts
// with `:key="note.id"` so each note gets its own editor — and its own undo history.
const { t } = useI18n()
const notesStore = useNotesStore()

const filter = ref('')
const confirmingDelete = ref<string | null>(null)
const editorRef = ref<{ flush: () => Promise<void>; discard: () => void } | null>(null)

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

async function confirmAndDelete(id: string) {
  if (confirmingDelete.value !== id) {
    // Two-press delete: the first press arms, the second acts.
    confirmingDelete.value = id
    setTimeout(() => {
      if (confirmingDelete.value === id) confirmingDelete.value = null
    }, 2500)
    return
  }
  confirmingDelete.value = null
  // The editor may be holding an unsaved edit for this note; discarding it stops the
  // unmount flush from writing the note back right after it was deleted.
  if (notesStore.selectedId === id) editorRef.value?.discard()
  await notesStore.deleteNote(id)
}

function switchNote(id: string) {
  confirmingDelete.value = null
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
      <div class="scroll">
        <p v-if="visibleNotes.length === 0" class="empty-pad">
          {{ filter.trim() ? t['notes.noMatch'] : t['notes.empty'] }}
        </p>
        <button
          v-for="note in visibleNotes"
          :key="note.id"
          type="button"
          class="note-item"
          :class="{ selected: note.id === notesStore.selectedId }"
          @click="switchNote(note.id)"
        >
          <span class="note-item-content">
            <span class="note-title">{{ notePreviewText(note.content, t['notes.untitled']) }}</span>
            <span class="note-date">{{ formatNoteDate(note.updated_at) }}</span>
          </span>
          <span
            class="delete-btn"
            :class="{ 'delete-btn--confirm': confirmingDelete === note.id }"
            role="button"
            tabindex="0"
            @click.stop="confirmAndDelete(note.id)"
          >
            <i
              v-if="confirmingDelete !== note.id"
              class="i-ri:delete-bin-5-line"
              aria-hidden="true"
            />
            <template v-else>{{ t['notes.delete'] }}?</template>
          </span>
        </button>
      </div>
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
  padding: 10px 12px;
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
  overflow-y: auto;
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
.delete-btn {
  visibility: hidden;
  margin-right: 6px;
  padding: 2px 6px;
  font-size: 11px;
  border-radius: 4px;
  color: var(--color-danger, #d03050);
}
.note-item:hover .delete-btn,
.delete-btn--confirm {
  visibility: visible;
}
.delete-btn:hover {
  background: var(--n-sidebar-hover);
}
.delete-btn--confirm {
  background: rgba(208, 48, 80, 0.15);
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
