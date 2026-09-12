<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from '~/i18n'
import { useNotesStore } from '~/stores/notes'
import { useEditor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'

const { t } = useI18n()
const notesStore = useNotesStore()

const saving = ref(false)
const saveTimer = ref<ReturnType<typeof setTimeout> | null>(null)
const title = ref('')
const dirty = ref(false)
const loadingContent = ref(false) // prevents dirty flag during setContent
const confirmingDelete = ref<string | null>(null)

// Extract title from first line of HTML content
function extractTitle(html: string): string {
  const text = html
    .slice(0, 400)
    .replace(/<[^>]+>/g, '\n')
    .trim()
  const lines = text.split('\n').filter(Boolean)
  const firstLine = lines[0] || 'Untitled'
  return firstLine.slice(0, 15) || t.value['notes.untitled']
}

// Build plain text preview (strip HTML) for sidebar
function textPreview(html: string, maxLen = 60): string {
  const text = html.replace(/<[^>]+>/g, '').trim()
  return text.length > maxLen ? text.slice(0, maxLen) + '…' : text || t.value['notes.empty']
}

function formatDate(ts: number): string {
  if (!ts) return ''
  // GORM stores timestamps in seconds; JS Date() expects milliseconds
  const date = new Date(ts * 1000)
  const now = new Date()
  const isToday = date.toDateString() === now.toDateString()
  if (isToday) {
    return date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
  }
  const isThisYear = date.getFullYear() === now.getFullYear()
  if (isThisYear) {
    return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
  }
  return date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

const editor = useEditor({
  content: '',
  extensions: [
    StarterKit.configure({
      heading: { levels: [1, 2, 3] },
    }),
  ],
  onUpdate: ({ editor: ed }) => {
    if (loadingContent.value) return
    const html = ed.getHTML()
    notesStore.currentContent = html
    title.value = extractTitle(html)
    dirty.value = true
    scheduleSave()
  },
})

function scheduleSave() {
  if (!notesStore.selectedId) return
  if (saveTimer.value) clearTimeout(saveTimer.value)
  saveTimer.value = setTimeout(doSave, 2000)
}

async function doSave() {
  if (!notesStore.selectedId) return
  saving.value = true
  try {
    await notesStore.saveCurrentNote(title.value, notesStore.currentContent)
    dirty.value = false
  } finally {
    saving.value = false
  }
}

function loadContentIntoEditor(html: string) {
  loadingContent.value = true
  editor.value?.commands.setContent(html || '')
  loadingContent.value = false
}

watch(
  () => notesStore.selectedId,
  (newId) => {
    if (!newId) {
      title.value = ''
      editor.value?.commands.setContent('')
      return
    }
    const note = notesStore.notes.find((n) => n.id === newId)
    if (note) {
      title.value = extractTitle(note.content || '')
      setTimeout(() => {
        loadContentIntoEditor(note.content || '')
        dirty.value = false
      }, 0)
    }
  },
)

async function handleNewNote() {
  const id = await notesStore.createNote()
  if (id) {
    title.value = ''
    editor.value?.commands.setContent('')
  }
}

async function confirmAndDelete(id: string) {
  if (confirmingDelete.value !== id) {
    confirmingDelete.value = id
    setTimeout(() => {
      if (confirmingDelete.value === id) confirmingDelete.value = null
    }, 2500)
    return
  }
  confirmingDelete.value = null
  if (notesStore.selectedId === id) {
    editor.value?.commands.setContent('')
    title.value = ''
  }
  await notesStore.deleteNote(id)
}

async function switchNote(id: string) {
  if (dirty.value && saveTimer.value) {
    clearTimeout(saveTimer.value)
    saveTimer.value = null
    await doSave()
  }
  confirmingDelete.value = null
  notesStore.selectNote(id)
}

onMounted(async () => {
  await notesStore.loadNotes()
  if (notesStore.selectedId) {
    const note = notesStore.notes.find((n) => n.id === notesStore.selectedId)
    if (note) {
      title.value = extractTitle(note.content || '')
      setTimeout(() => {
        loadContentIntoEditor(note.content || '')
        dirty.value = false
      }, 0)
    }
  }
})

onUnmounted(() => {
  if (saveTimer.value) clearTimeout(saveTimer.value)
  editor.value?.destroy()
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
      <div class="scroll">
        <p v-if="notesStore.sortedNotes.length === 0" class="empty-pad">
          {{ t['notes.empty'] }}
        </p>
        <button
          v-for="note in notesStore.sortedNotes"
          :key="note.id"
          type="button"
          class="note-item"
          :class="{ selected: note.id === notesStore.selectedId }"
          @click="switchNote(note.id)"
        >
          <span class="note-item-content">
            <span class="note-title">{{ textPreview(note.content) }}</span>
            <span class="note-date">{{ formatDate(note.updated_at) }}</span>
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

    <!-- Editor -->
    <div class="notes-editor">
      <div class="editor-toolbar" v-if="notesStore.selectedId">
        <button
          type="button"
          class="tb"
          :class="{ active: editor?.isActive('bold') }"
          title="Bold"
          @click="editor?.chain().focus().toggleBold().run()"
        >
          <i class="i-ri:bold" aria-hidden="true" />
        </button>
        <button
          type="button"
          class="tb"
          :class="{ active: editor?.isActive('italic') }"
          title="Italic"
          @click="editor?.chain().focus().toggleItalic().run()"
        >
          <i class="i-ri:italic" aria-hidden="true" />
        </button>
        <button
          type="button"
          class="tb"
          :class="{ active: editor?.isActive('strike') }"
          title="Strike"
          @click="editor?.chain().focus().toggleStrike().run()"
        >
          <i class="i-ri:strikethrough" aria-hidden="true" />
        </button>
        <span class="toolbar-sep" />
        <button
          v-for="lvl in [1, 2, 3] as const"
          :key="lvl"
          type="button"
          class="tb tb--text select-none"
          :class="{ active: editor?.isActive('heading', { level: lvl }) }"
          title="Heading H{{ lvl }}"
          @click="editor?.chain().focus().toggleHeading({ level: lvl }).run()"
        >
          H{{ lvl }}
        </button>
        <span class="toolbar-sep" />
        <button
          type="button"
          class="tb"
          :class="{ active: editor?.isActive('bulletList') }"
          title="Bullet list"
          @click="editor?.chain().focus().toggleBulletList().run()"
        >
          <i class="i-ri:list-unordered" aria-hidden="true" />
        </button>
        <button
          type="button"
          class="tb"
          :class="{ active: editor?.isActive('orderedList') }"
          title="Numbered list"
          @click="editor?.chain().focus().toggleOrderedList().run()"
        >
          <i class="i-ri:list-ordered" aria-hidden="true" />
        </button>
        <button
          type="button"
          class="tb"
          :class="{ active: editor?.isActive('blockquote') }"
          title="Quote"
          @click="editor?.chain().focus().toggleBlockquote().run()"
        >
          <i class="i-ri:double-quotes-l" aria-hidden="true" />
        </button>
        <button
          type="button"
          class="tb"
          :class="{ active: editor?.isActive('codeBlock') }"
          title="Code block"
          @click="editor?.chain().focus().toggleCodeBlock().run()"
        >
          <i class="i-ri:code-box-line" aria-hidden="true" />
        </button>
        <span class="toolbar-sep" />
        <button type="button" class="tb" title="Undo" @click="editor?.chain().focus().undo().run()">
          <i class="i-ri:arrow-go-back-line" aria-hidden="true" />
        </button>
        <button type="button" class="tb" title="Redo" @click="editor?.chain().focus().redo().run()">
          <i class="i-ri:arrow-go-forward-line" aria-hidden="true" />
        </button>
      </div>

      <div class="editor-content-wrapper" v-if="notesStore.selectedId">
        <EditorContent :editor="editor" class="editor-content" />
      </div>

      <div v-else class="editor-placeholder">
        <p class="empty-pad">{{ t['notes.empty'] }}</p>
      </div>
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
  background: transparent;
  margin: 2px 0;
  padding: 0;
  border-left: 2px solid transparent;
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

.notes-editor {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  height: 100%;
}
.editor-toolbar {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 6px 8px;
  border-bottom: 1px solid var(--n-border-color);
  flex-shrink: 0;
  flex-wrap: wrap;
}
.tb {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 24px;
  padding: 0;
  border: none;
  background: transparent;
  border-radius: 4px;
  cursor: pointer;
  color: var(--n-text-color);
}
.tb:hover {
  background: var(--n-sidebar-hover);
}
.tb--text {
  width: auto;
  padding: 0 6px;
  font-size: 11px;
  font-weight: 600;
}
.tb i {
  font-size: 16px;
}
.tb.active {
  color: var(--n-primary-color);
  background: var(--n-sidebar-active);
}
.toolbar-sep {
  width: 1px;
  height: 18px;
  background: var(--n-border-color);
  margin: 0 3px;
  flex-shrink: 0;
}

.editor-content-wrapper {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}
.editor-content {
  padding: 20px 24px;
  outline: none;
  color: var(--n-text-color);
}
.editor-content :deep(.ProseMirror) {
  outline: none;
  min-height: 200px;
  line-height: 1.7;
  font-size: 14px;
}
.editor-content :deep(.ProseMirror p) {
  margin: 0 0 6px;
}
.editor-content :deep(.ProseMirror h1) {
  font-size: 22px;
  font-weight: 700;
  margin: 16px 0 8px;
}
.editor-content :deep(.ProseMirror h2) {
  font-size: 18px;
  font-weight: 600;
  margin: 12px 0 6px;
}
.editor-content :deep(.ProseMirror h3) {
  font-size: 16px;
  font-weight: 600;
  margin: 10px 0 4px;
}
.editor-content :deep(.ProseMirror ul),
.editor-content :deep(.ProseMirror ol) {
  padding-left: 24px;
  margin: 4px 0;
}
.editor-content :deep(.ProseMirror blockquote) {
  margin: 8px 0;
  padding: 0 0 0 12px;
  border-left: 3px solid var(--n-border-color);
  opacity: 0.85;
}
.editor-content :deep(.ProseMirror pre) {
  background: var(--n-action-color);
  padding: 12px;
  border-radius: 4px;
  overflow-x: auto;
  margin: 8px 0;
}
.editor-content :deep(.ProseMirror code) {
  font-family: 'SF Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 13px;
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
