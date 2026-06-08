<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from '../i18n'
import { useNotesStore } from '../stores/notes'
import { NButton, NIcon, NEmpty, NScrollbar, NList, NListItem, NPopconfirm } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { useEditor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import { useThemeVars } from 'naive-ui'

const themeVars = useThemeVars()
const { primaryColor, codeColor, borderColor, actionColor } = themeVars.value

const { t } = useI18n()
const notesStore = useNotesStore()

const saving = ref(false)
const saveTimer = ref<ReturnType<typeof setTimeout> | null>(null)
const title = ref('')
const dirty = ref(false)
const loadingContent = ref(false) // prevents dirty flag during setContent

// Extract title from first line of HTML content
function extractTitle(html: string): string {
  const text = html
    .slice(0, 400)
    .replace(/<[^>]+>/g, '\n')
    .trim()
  const lines = text.split('\n').filter(Boolean)
  const firstLine = lines[0]
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

// Editor — useEditor already returns a Ref
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
  } finally {
    saving.value = false
  }
}

/** Load note content — use setContent which creates a single transaction boundary */
function loadContentIntoEditor(html: string) {
  loadingContent.value = true
  // setContent replaces the whole doc in one transaction. Undo from here
  // goes back to empty document, NOT to the previous note's content.
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

async function handleDelete(id: string) {
  if (notesStore.selectedId === id) {
    editor.value?.commands.setContent('')
    title.value = ''
  }
  await notesStore.deleteNote(id)
}

/** Save current note before switching to another — only if content changed */
async function switchNote(id: string) {
  if (dirty.value && saveTimer.value) {
    clearTimeout(saveTimer.value)
    saveTimer.value = null
    await doSave()
  }
  notesStore.selectNote(id)
}

onMounted(async () => {
  await notesStore.loadNotes()
  // If selectedId is already set (survived route change via Pinia),
  // load the content into the editor now that notes are fetched.
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
        <NButton size="tiny" type="primary" @click="handleNewNote">
          <template #icon>
            <NIcon><Icon icon="ri:pen-nib-fill" /></NIcon>
          </template>
          {{ t['notes.new'] }}
        </NButton>
      </div>

      <NScrollbar style="height: 100%">
        <NEmpty
          v-if="notesStore.sortedNotes.length === 0"
          :description="t['notes.empty']"
          size="small"
          style="padding: 24px 12px"
        />
        <NList v-else>
          <NListItem
            v-for="note in notesStore.sortedNotes"
            :key="note.id"
            class="note-item"
            :class="{ selected: note.id === notesStore.selectedId }"
            @click="switchNote(note.id)"
          >
            <div class="note-item-content">
              <div class="note-title">{{ textPreview(note.content) }}</div>
              <div class="note-date">{{ formatDate(note.updated_at) }}</div>
            </div>
            <template #suffix>
              <NPopconfirm
                :positive-text="t['notes.delete']"
                :negative-text="t['settings.accounts.cancel']"
                @positive-click="handleDelete(note.id)"
              >
                <template #trigger>
                  <NButton size="tiny" quaternary type="error" class="delete-btn">
                    <template #icon>
                      <NIcon size="14"><Icon icon="ri:delete-bin-5-line" /></NIcon>
                    </template>
                  </NButton>
                </template>
                {{ t['notes.delete'] }}?
              </NPopconfirm>
            </template>
          </NListItem>
        </NList>
      </NScrollbar>
    </div>

    <!-- Editor -->
    <div class="notes-editor">
      <div class="editor-toolbar" v-if="notesStore.selectedId">
        <NButton
          size="tiny"
          quaternary
          :class="{ active: editor?.isActive('bold') }"
          @click="editor?.chain().focus().toggleBold().run()"
        >
          <NIcon><Icon icon="ri:bold" /></NIcon>
        </NButton>
        <NButton
          size="tiny"
          quaternary
          :class="{ active: editor?.isActive('italic') }"
          @click="editor?.chain().focus().toggleItalic().run()"
        >
          <NIcon><Icon icon="ri:italic" /></NIcon>
        </NButton>
        <NButton
          size="tiny"
          quaternary
          :class="{ active: editor?.isActive('strike') }"
          @click="editor?.chain().focus().toggleStrike().run()"
        >
          <NIcon><Icon icon="ri:strikethrough" /></NIcon>
        </NButton>
        <span class="toolbar-sep" />
        <NButton
          size="tiny"
          quaternary
          :class="{ active: editor?.isActive('heading', { level: 1 }) }"
          @click="editor?.chain().focus().toggleHeading({ level: 1 }).run()"
        >
          <NIcon><Icon icon="ci:heading-h1" /></NIcon>
        </NButton>
        <NButton
          size="tiny"
          quaternary
          :class="{ active: editor?.isActive('heading', { level: 2 }) }"
          @click="editor?.chain().focus().toggleHeading({ level: 2 }).run()"
        >
          <NIcon><Icon icon="ci:heading-h2" /></NIcon>
        </NButton>
        <NButton
          size="tiny"
          quaternary
          :class="{ active: editor?.isActive('heading', { level: 3 }) }"
          @click="editor?.chain().focus().toggleHeading({ level: 3 }).run()"
        >
          <NIcon><Icon icon="ci:heading-h3" /></NIcon>
        </NButton>
        <span class="toolbar-sep" />
        <NButton
          size="tiny"
          quaternary
          :class="{ active: editor?.isActive('bulletList') }"
          @click="editor?.chain().focus().toggleBulletList().run()"
        >
          <NIcon><Icon icon="ri:list-unordered" /></NIcon>
        </NButton>
        <NButton
          size="tiny"
          quaternary
          :class="{ active: editor?.isActive('orderedList') }"
          @click="editor?.chain().focus().toggleOrderedList().run()"
        >
          <NIcon><Icon icon="ri:list-ordered" /></NIcon>
        </NButton>
        <NButton
          size="tiny"
          quaternary
          :class="{ active: editor?.isActive('blockquote') }"
          @click="editor?.chain().focus().toggleBlockquote().run()"
        >
          <NIcon><Icon icon="ri:double-quotes-l" /></NIcon>
        </NButton>
        <NButton
          size="tiny"
          quaternary
          :class="{ active: editor?.isActive('codeBlock') }"
          @click="editor?.chain().focus().toggleCodeBlock().run()"
        >
          <NIcon><Icon icon="ri:code-box-line" /></NIcon>
        </NButton>
        <span class="toolbar-sep" />
        <NButton size="tiny" quaternary @click="editor?.chain().focus().undo().run()">
          <NIcon><Icon icon="ri:arrow-go-back-line" /></NIcon>
        </NButton>
        <NButton size="tiny" quaternary @click="editor?.chain().focus().redo().run()">
          <NIcon><Icon icon="ri:arrow-go-forward-line" /></NIcon>
        </NButton>
      </div>

      <div class="editor-content-wrapper" v-if="notesStore.selectedId">
        <EditorContent :editor="editor" class="editor-content" />
      </div>

      <div v-else class="editor-placeholder">
        <NEmpty :description="t['notes.empty']" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.notes-view {
  height: 100%;
  display: flex;
  flex-direction: row;
}

.notes-sidebar {
  width: 260px;
  height: 100%;
  display: flex;
  flex-direction: column;
  border-right: 1px solid v-bind(borderColor);
  flex-shrink: 0;
}

.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px;
  border-bottom: 1px solid v-bind(borderColor);
}

.sidebar-title {
  font-size: 14px;
  font-weight: 600;
}

.note-item {
  cursor: pointer;
  border-left: 3px solid transparent;
  /* background-color: v-bind(actionColor); */
}

.note-item:hover {
  background-color: v-bind(actionColor);
}

.note-item.selected {
  border-left-color: v-bind(primaryColor);
}

.note-item-content {
  padding: 8px 8px;
  min-width: 0;
}

.note-title {
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.note-date {
  font-size: 11px;
  opacity: 0.5;
  margin-top: 2px;
}

.delete-btn {
  opacity: 0;
  transition: opacity 0.15s;
}

.note-item:hover .delete-btn {
  opacity: 1;
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
  padding: 8px 16px;
  border-bottom: 1px solid v-bind(borderColor);
  flex-shrink: 0;
  flex-wrap: wrap;
}

.editor-toolbar .active {
  color: v-bind(primaryColor);
  background-color: v-bind(codeColor);
}

.toolbar-sep {
  width: 1px;
  height: 20px;
  background: v-bind(borderColor);
  margin: 0 4px;
  flex-shrink: 0;
}

.editor-content-wrapper {
  flex: 1;
  overflow-y: auto;
}

.editor-content {
  padding: 20px 24px;
  min-height: 100%;
  outline: none;
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
  border-left: 3px solid v-bind(borderColor);
  opacity: 0.8;
}

.editor-content :deep(.ProseMirror pre) {
  background: v-bind(codeColor);
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
</style>
