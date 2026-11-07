<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { EditorContent, useEditor } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Image from '@tiptap/extension-image'
import { BackgroundColor, Color, TextStyle } from '@tiptap/extension-text-style'
import { useI18n } from '~/i18n'
import UiScrollArea from '~/components/ui/UiScrollArea.vue'
import UiRichTextToolbar from '~/components/ui/UiRichTextToolbar.vue'
import UiTooltip from '~/components/ui/UiTooltip.vue'

const props = defineProps<{
  modelValue: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  'update:text': [value: string]
  'pick-attachment': []
  'pick-image': []
}>()

const { t, tf } = useI18n()
const editor = useEditor({
  content: props.modelValue || '<p></p>',
  extensions: [
    StarterKit.configure({
      heading: { levels: [1, 2, 3] },
    }),
    Image.configure({
      inline: true,
      allowBase64: true,
      HTMLAttributes: {
        class: 'compose-inline-image',
      },
    }).extend({
      addAttributes() {
        return {
          ...this.parent?.(),
          composeImageId: {
            default: null,
            parseHTML: (element) => element.getAttribute('data-compose-image-id'),
            renderHTML: (attributes) =>
              attributes.composeImageId
                ? { 'data-compose-image-id': attributes.composeImageId }
                : {},
          },
          contentId: {
            default: null,
            parseHTML: (element) => element.getAttribute('data-content-id'),
            renderHTML: (attributes) =>
              attributes.contentId ? { 'data-content-id': attributes.contentId } : {},
          },
        }
      },
    }),
    TextStyle,
    Color,
    BackgroundColor,
  ],
  onUpdate: ({ editor: ed }) => {
    emit('update:modelValue', ed.getHTML())
    emit('update:text', ed.getText({ blockSeparator: '\n\n' }))
  },
})

watch(
  () => props.modelValue,
  (value) => {
    if (!editor.value || value === editor.value.getHTML()) return
    editor.value.commands.setContent(value || '<p></p>', { emitUpdate: false })
  },
)

function insertImage(image: { id: string; preview: string; name: string; contentId: string }) {
  editor.value
    ?.chain()
    .focus()
    .insertContent({
      type: 'image',
      attrs: {
        src: image.preview,
        alt: image.name,
        title: image.name,
        composeImageId: image.id,
        contentId: image.contentId,
      },
    })
    .run()
}

function focus() {
  editor.value?.commands.focus()
}

defineExpose({ insertImage, focus })

onBeforeUnmount(() => {
  editor.value?.destroy()
})
</script>

<template>
  <div class="compose-editor">
    <div class="compose-toolbar" role="toolbar" :aria-label="t['compose.editor_toolbar']">
      <UiTooltip :label="t['notes.bold']">
        <button
          type="button"
          class="tb"
          :class="{ active: editor?.isActive('bold') }"
          :aria-label="t['notes.bold']"
          @click="editor?.chain().focus().toggleBold().run()"
        >
          <i class="i-ri:bold" aria-hidden="true" />
        </button>
      </UiTooltip>
      <UiTooltip :label="t['notes.italic']">
        <button
          type="button"
          class="tb"
          :class="{ active: editor?.isActive('italic') }"
          :aria-label="t['notes.italic']"
          @click="editor?.chain().focus().toggleItalic().run()"
        >
          <i class="i-ri:italic" aria-hidden="true" />
        </button>
      </UiTooltip>
      <UiTooltip :label="t['compose.underline']">
        <button
          type="button"
          class="tb"
          :class="{ active: editor?.isActive('underline') }"
          :aria-label="t['compose.underline']"
          @click="editor?.chain().focus().toggleUnderline().run()"
        >
          <i class="i-ri:underline" aria-hidden="true" />
        </button>
      </UiTooltip>
      <UiTooltip :label="t['notes.strike']">
        <button
          type="button"
          class="tb"
          :class="{ active: editor?.isActive('strike') }"
          :aria-label="t['notes.strike']"
          @click="editor?.chain().focus().toggleStrike().run()"
        >
          <i class="i-ri:strikethrough" aria-hidden="true" />
        </button>
      </UiTooltip>
      <span class="toolbar-sep" />
      <UiTooltip
        v-for="lvl in [1, 2, 3] as const"
        :key="lvl"
        :label="tf('notes.heading', { level: lvl })"
      >
        <button
          type="button"
          class="tb tb--text"
          :class="{ active: editor?.isActive('heading', { level: lvl }) }"
          :aria-label="tf('notes.heading', { level: lvl })"
          @click="editor?.chain().focus().toggleHeading({ level: lvl }).run()"
        >
          H{{ lvl }}
        </button>
      </UiTooltip>
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
      <UiTooltip :label="t['notes.bulletList']">
        <button
          type="button"
          class="tb"
          :class="{ active: editor?.isActive('bulletList') }"
          :aria-label="t['notes.bulletList']"
          @click="editor?.chain().focus().toggleBulletList().run()"
        >
          <i class="i-ri:list-unordered" aria-hidden="true" />
        </button>
      </UiTooltip>
      <UiTooltip :label="t['notes.orderedList']">
        <button
          type="button"
          class="tb"
          :class="{ active: editor?.isActive('orderedList') }"
          :aria-label="t['notes.orderedList']"
          @click="editor?.chain().focus().toggleOrderedList().run()"
        >
          <i class="i-ri:list-ordered" aria-hidden="true" />
        </button>
      </UiTooltip>
      <UiTooltip :label="t['notes.quote']">
        <button
          type="button"
          class="tb"
          :class="{ active: editor?.isActive('blockquote') }"
          :aria-label="t['notes.quote']"
          @click="editor?.chain().focus().toggleBlockquote().run()"
        >
          <i class="i-ri:double-quotes-l" aria-hidden="true" />
        </button>
      </UiTooltip>
      <UiTooltip :label="t['notes.codeBlock']">
        <button
          type="button"
          class="tb"
          :class="{ active: editor?.isActive('codeBlock') }"
          :aria-label="t['notes.codeBlock']"
          @click="editor?.chain().focus().toggleCodeBlock().run()"
        >
          <i class="i-ri:code-box-line" aria-hidden="true" />
        </button>
      </UiTooltip>
      <span class="toolbar-sep" />
      <UiTooltip :label="t['compose.insert_image']">
        <button
          type="button"
          class="tb"
          :aria-label="t['compose.insert_image']"
          @click="emit('pick-image')"
        >
          <i class="i-ri:image-add-line" aria-hidden="true" />
        </button>
      </UiTooltip>
      <UiTooltip :label="t['compose.attach']">
        <button
          type="button"
          class="tb"
          :aria-label="t['compose.attach']"
          @click="emit('pick-attachment')"
        >
          <i class="i-ri:attachment-2" aria-hidden="true" />
        </button>
      </UiTooltip>
      <span class="toolbar-sep" />
      <UiTooltip :label="t['notes.undo']">
        <button
          type="button"
          class="tb"
          :aria-label="t['notes.undo']"
          :disabled="!editor?.can().undo()"
          @click="editor?.chain().focus().undo().run()"
        >
          <i class="i-ri:arrow-go-back-line" aria-hidden="true" />
        </button>
      </UiTooltip>
      <UiTooltip :label="t['notes.redo']">
        <button
          type="button"
          class="tb"
          :aria-label="t['notes.redo']"
          :disabled="!editor?.can().redo()"
          @click="editor?.chain().focus().redo().run()"
        >
          <i class="i-ri:arrow-go-forward-line" aria-hidden="true" />
        </button>
      </UiTooltip>
    </div>

    <UiScrollArea class="editor-scroll">
      <EditorContent :editor="editor" class="editor-content" />
    </UiScrollArea>
  </div>
</template>

<style scoped>
.compose-editor {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
}
.compose-toolbar {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 2px;
  min-height: 38px;
  padding: 4px 12px;
  overflow-x: auto;
  overflow-y: hidden;
  border-bottom: 1px solid var(--n-border-color);
  scrollbar-width: none;
}
.compose-toolbar::-webkit-scrollbar {
  display: none;
}
.tb {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  color: var(--color-text-2);
  background: transparent;
  border: 1px solid transparent;
  border-radius: 5px;
  cursor: pointer;
}
.tb:hover:not(:disabled),
.tb.active {
  color: var(--n-text-color);
  background: var(--n-sidebar-hover);
}
.tb.active {
  border-color: color-mix(in srgb, var(--n-primary-color) 32%, transparent);
  color: var(--n-primary-color);
}
.tb:disabled {
  opacity: 0.35;
  cursor: default;
}
.tb i {
  font-size: 16px;
}
.tb--text {
  width: auto;
  padding: 0 5px;
  font-size: 10.5px;
  font-weight: 650;
}
.toolbar-sep {
  width: 1px;
  height: 16px;
  margin: 0 4px;
  background: var(--n-border-color);
}
.editor-scroll {
  flex: 1;
  min-height: 0;
}
.editor-content {
  min-height: 100%;
  display: flex;
  flex-direction: column;
  color: var(--n-text-color);
}
.editor-content :deep(.ProseMirror) {
  flex: 1;
  min-height: 200px;
  padding: 18px 24px 48px;
  outline: none;
  cursor: text;
  font-size: 14px;
  line-height: 1.72;
}
.editor-content :deep(.ProseMirror > :first-child) {
  margin-top: 0;
}
.editor-content :deep(.ProseMirror p) {
  margin: 0 0 0.85em;
}
.editor-content :deep(.ProseMirror h1),
.editor-content :deep(.ProseMirror h2),
.editor-content :deep(.ProseMirror h3) {
  margin: 1.1em 0 0.45em;
  line-height: 1.25;
}
.editor-content :deep(.ProseMirror h1) {
  font-size: 24px;
}
.editor-content :deep(.ProseMirror h2) {
  font-size: 20px;
}
.editor-content :deep(.ProseMirror h3) {
  font-size: 17px;
}
.editor-content :deep(.ProseMirror blockquote) {
  margin: 0.8em 0;
  padding-left: 12px;
  border-left: 3px solid var(--n-primary-color);
  color: var(--color-text-2);
}
.editor-content :deep(.ProseMirror pre) {
  padding: 10px 12px;
  overflow: auto;
  border-radius: 6px;
  background: var(--n-action-color);
}
.editor-content :deep(.ProseMirror img) {
  display: inline-block;
  max-width: min(100%, 560px);
  max-height: 360px;
  margin: 6px 0;
  border-radius: 6px;
  object-fit: contain;
  vertical-align: middle;
}
.editor-content :deep(.ProseMirror a) {
  color: var(--n-primary-color);
  text-decoration: underline;
}
.editor-content :deep(.ProseMirror ul),
.editor-content :deep(.ProseMirror ol) {
  padding-left: 1.6em;
}
</style>
