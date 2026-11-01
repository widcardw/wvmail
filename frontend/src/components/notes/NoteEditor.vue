<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { EditorContent, useEditor } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import { BackgroundColor, Color, TextStyle } from '@tiptap/extension-text-style'
import { useI18n } from '~/i18n'
import { useNotesStore } from '~/stores/notes'
import { useAccountsStore } from '~/stores/accounts'
import { useFeedbackStore } from '~/stores/feedback'
import { extractNoteTitle } from '~/composables/note-text'
import UiScrollArea from '~/components/ui/UiScrollArea.vue'
import UiRichTextToolbar from '~/components/ui/UiRichTextToolbar.vue'
import type { Note } from '~/stores/notes'
import { ComposeService } from '#/wvmail/services'

// The editing half of a note: the TipTap instance, its toolbar, and the debounced
// autosave. The parent (NotesView) mounts this with `:key="note.id"`.
//
// That key is load-bearing, not cosmetic. TipTap's `setContent()` dispatches an
// ordinary, *undoable* transaction, so reusing one editor and pushing a new note into
// it keeps the previous note's history: the first Ctrl+Z (or the toolbar's undo)
// after a switch would restore the previous note's text into the current one, and the
// autosave would then persist that mix. A fresh instance per note makes the history
// per note by construction.
const props = defineProps<{ note: Note }>()

const SAVE_DEBOUNCE_MS = 2000

const { t, tf } = useI18n()
const notesStore = useNotesStore()
const accountsStore = useAccountsStore()
const feedback = useFeedbackStore()

const title = ref(extractNoteTitle(props.note.content || '', t.value['notes.untitled']))
const dirty = ref(false)
const saving = ref(false)
const openingCompose = ref(false)
/** Latest editor HTML; the debounce saves this, not a re-read of the editor. */
const pendingHtml = ref(props.note.content || '')
/** Set by the parent when the note is being deleted: a pending save must not resurrect it. */
let discarded = false
let saveTimer: ReturnType<typeof setTimeout> | null = null

const editor = useEditor({
  content: props.note.content || '',
  extensions: [
    StarterKit.configure({
      heading: { levels: [1, 2, 3] },
    }),
    TextStyle,
    Color,
    BackgroundColor,
  ],
  onUpdate: ({ editor: ed }) => {
    const html = ed.getHTML()
    pendingHtml.value = html
    title.value = extractNoteTitle(html, t.value['notes.untitled'])
    dirty.value = true
    if (saveTimer) clearTimeout(saveTimer)
    saveTimer = setTimeout(() => void save(), SAVE_DEBOUNCE_MS)
  },
})

async function save() {
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveTimer = null
  }
  if (!dirty.value) return
  saving.value = true
  try {
    // Saved by the note's own id, never by "the current selection": the switch that
    // unmounts this instance has already moved the selection on.
    await notesStore.saveNote(props.note.id, title.value, pendingHtml.value)
    dirty.value = false
  } finally {
    saving.value = false
  }
}

/** Write the pending edit now (called by the parent before it changes the selection). */
async function flush() {
  if (!discarded) await save()
}

/** Drop the pending edit: the note is going away, so saving would re-create it. */
function discard() {
  discarded = true
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveTimer = null
  }
  dirty.value = false
}

async function sendToCompose() {
  if (openingCompose.value) return
  openingCompose.value = true
  try {
    await flush()
    const result = await ComposeService.OpenCompose({
      AccountID:
        accountsStore.selectedEmailAccountId ??
        accountsStore.selectedAccountId ??
        accountsStore.accounts[0]?.ID ??
        '',
      To: [],
      Cc: [],
      Bcc: [],
      Subject: title.value.trim() || t.value['notes.untitled'],
      Text: '',
      HTML: pendingHtml.value || '<p></p>',
      InReplyTo: '',
      References: [],
    })
    if (result.Warning) feedback.info(result.Warning)
  } catch (e) {
    console.error('Failed to open compose for note', e)
    feedback.error(t.value['notes.send_to_failed'])
  } finally {
    openingCompose.value = false
  }
}

onBeforeUnmount(() => {
  // Leaving the note (switch, route change): do not lose the last edits. This runs
  // with the *old* note still in `props`, so the save targets the right row.
  void flush()
  editor.value?.destroy()
})

defineExpose({ flush, discard })
</script>

<template>
  <div class="notes-editor">
    <div class="editor-toolbar">
      <button
        type="button"
        class="tb"
        :class="{ active: editor?.isActive('bold') }"
        :title="t['notes.bold']"
        @click="editor?.chain().focus().toggleBold().run()"
      >
        <i class="i-ri:bold" aria-hidden="true" />
      </button>
      <button
        type="button"
        class="tb"
        :class="{ active: editor?.isActive('italic') }"
        :title="t['notes.italic']"
        @click="editor?.chain().focus().toggleItalic().run()"
      >
        <i class="i-ri:italic" aria-hidden="true" />
      </button>
      <button
        type="button"
        class="tb"
        :class="{ active: editor?.isActive('underline') }"
        :title="t['compose.underline']"
        @click="editor?.chain().focus().toggleUnderline().run()"
      >
        <i class="i-ri:underline" aria-hidden="true" />
      </button>
      <button
        type="button"
        class="tb"
        :class="{ active: editor?.isActive('strike') }"
        :title="t['notes.strike']"
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
        :title="tf('notes.heading', { level: lvl })"
        @click="editor?.chain().focus().toggleHeading({ level: lvl }).run()"
      >
        H{{ lvl }}
      </button>
      <span class="toolbar-sep" />
      <UiRichTextToolbar
        :editor="editor"
        :text-color-label="t['compose.text_color']"
        :background-color-label="t['compose.background_color']"
        :custom-color-label="t['compose.color_custom']"
        :default-color-label="t['compose.color_default']"
        :link-label="t['compose.link']"
        :link-prompt="t['compose.link_prompt']"
        :cancel-label="t['cancel']"
      />
      <span class="toolbar-sep" />
      <button
        type="button"
        class="tb"
        :class="{ active: editor?.isActive('bulletList') }"
        :title="t['notes.bulletList']"
        @click="editor?.chain().focus().toggleBulletList().run()"
      >
        <i class="i-ri:list-unordered" aria-hidden="true" />
      </button>
      <button
        type="button"
        class="tb"
        :class="{ active: editor?.isActive('orderedList') }"
        :title="t['notes.orderedList']"
        @click="editor?.chain().focus().toggleOrderedList().run()"
      >
        <i class="i-ri:list-ordered" aria-hidden="true" />
      </button>
      <button
        type="button"
        class="tb"
        :class="{ active: editor?.isActive('blockquote') }"
        :title="t['notes.quote']"
        @click="editor?.chain().focus().toggleBlockquote().run()"
      >
        <i class="i-ri:double-quotes-l" aria-hidden="true" />
      </button>
      <button
        type="button"
        class="tb"
        :class="{ active: editor?.isActive('codeBlock') }"
        :title="t['notes.codeBlock']"
        @click="editor?.chain().focus().toggleCodeBlock().run()"
      >
        <i class="i-ri:code-box-line" aria-hidden="true" />
      </button>
      <span class="toolbar-sep" />
      <!-- Disabled when this note has nothing to undo/redo: after a switch the history
           is empty, so the buttons are visibly inert instead of silently restoring
           something from another note. -->
      <button
        type="button"
        class="tb"
        :title="t['notes.undo']"
        :disabled="!editor?.can().undo()"
        @click="editor?.chain().focus().undo().run()"
      >
        <i class="i-ri:arrow-go-back-line" aria-hidden="true" />
      </button>
      <button
        type="button"
        class="tb"
        :title="t['notes.redo']"
        :disabled="!editor?.can().redo()"
        @click="editor?.chain().focus().redo().run()"
      >
        <i class="i-ri:arrow-go-forward-line" aria-hidden="true" />
      </button>
      <span class="toolbar-spacer" />
      <button
        type="button"
        class="tb"
        :title="t['notes.send_to']"
        :aria-label="t['notes.send_to']"
        :disabled="openingCompose"
        @click="sendToCompose"
      >
        <i class="i-ri:mail-send-line" aria-hidden="true" />
      </button>
      <span v-if="saving || dirty" class="save-state">
        {{ saving ? t['notes.saving'] : t['notes.unsaved'] }}
      </span>
    </div>

    <UiScrollArea class="editor-content-wrapper">
      <EditorContent :editor="editor" class="editor-content" />
    </UiScrollArea>
  </div>
</template>

<style scoped>
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
  height: var(--pane-header-height);
  padding: 0 12px;
  border-bottom: 1px solid var(--n-border-color);
  flex-shrink: 0;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: none;
}
.editor-toolbar::-webkit-scrollbar {
  display: none;
}
.tb {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 26px;
  height: 24px;
  padding: 0;
  border: none;
  background: transparent;
  border-radius: 4px;
  cursor: pointer;
  color: var(--n-text-color);
}
.tb:hover:not(:disabled) {
  background: var(--n-sidebar-hover);
}
.tb:disabled {
  opacity: 0.35;
  cursor: default;
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
.toolbar-spacer {
  flex: 1;
}
.save-state {
  padding-right: 4px;
  font-size: 11px;
  color: var(--color-text-3);
}

.editor-content-wrapper {
  flex: 1;
  min-height: 0;
}
.editor-content {
  /* At least the pane's height, so the editable box reaches the bottom; a taller note
     still grows past it and the wrapper scrolls. */
  min-height: 100%;
  display: flex;
  flex-direction: column;
  outline: none;
  color: var(--n-text-color);
}
/* The editable element *is* the pane: without a height of its own (the old
   `min-height: 200px` only) the space under the text was not editable, so clicking
   there never showed a caret. The padding lives here rather than on the wrapper so
   clicking the padding focuses the editor too. */
.editor-content :deep(.ProseMirror) {
  flex: 1;
  min-height: 200px;
  padding: 20px 24px;
  outline: none;
  /* The whole filled area is editable, so it should look editable everywhere —
     including the padding below the last line. */
  cursor: text;
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
  padding-left: 12px;
  border-left: 3px solid var(--n-border-color);
  color: var(--color-text-2);
}
.editor-content :deep(.ProseMirror pre) {
  background: var(--n-action-color);
  padding: 12px;
  border-radius: 4px;
  overflow-x: auto;
  margin: 8px 0;
}
.editor-content :deep(.ProseMirror code) {
  font-family: var(--font-mono);
  font-size: 13px;
}
</style>
