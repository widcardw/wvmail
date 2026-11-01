<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { EditorContent, useEditor } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Image from '@tiptap/extension-image'
import { useI18n } from '~/i18n'
import UiScrollArea from '~/components/ui/UiScrollArea.vue'

const props = defineProps<{
  modelValue: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  'update:text': [value: string]
  'pick-image': []
}>()

const { t } = useI18n()
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

function setLink() {
  const url = window.prompt(t.value['compose.link_prompt'])
  if (!url) return
  editor.value?.chain().focus().extendMarkRange('link').setLink({ href: url }).run()
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
      <button
        type="button"
        class="tb"
        :class="{ active: editor?.isActive('link') }"
        :title="t['compose.link']"
        @click="setLink"
      >
        <i class="i-ri:link" aria-hidden="true" />
      </button>
      <button type="button" class="tb" :title="t['compose.insert_image']" @click="emit('pick-image')">
        <i class="i-ri:image-add-line" aria-hidden="true" />
      </button>
      <span class="toolbar-sep" />
      <button
        type="button"
        class="tb"
        :disabled="!editor?.can().undo()"
        :title="t['notes.undo']"
        @click="editor?.chain().focus().undo().run()"
      >
        <i class="i-ri:arrow-go-back-line" aria-hidden="true" />
      </button>
      <button
        type="button"
        class="tb"
        :disabled="!editor?.can().redo()"
        :title="t['notes.redo']"
        @click="editor?.chain().focus().redo().run()"
      >
        <i class="i-ri:arrow-go-forward-line" aria-hidden="true" />
      </button>
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
  border-bottom: 1px solid var(--n-border-color);
  background: color-mix(in srgb, var(--n-body-color) 92%, var(--n-primary-color));
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
  padding: 18px 24px 48px;
}
.editor-content :deep(.tiptap) {
  min-height: 100%;
  outline: none;
  font-size: 14px;
  line-height: 1.72;
}
.editor-content :deep(.tiptap > :first-child) {
  margin-top: 0;
}
.editor-content :deep(.tiptap p) {
  margin: 0 0 0.85em;
}
.editor-content :deep(.tiptap h1),
.editor-content :deep(.tiptap h2),
.editor-content :deep(.tiptap h3) {
  margin: 1.1em 0 0.45em;
  line-height: 1.25;
}
.editor-content :deep(.tiptap blockquote) {
  margin: 0.8em 0;
  padding-left: 12px;
  border-left: 3px solid var(--n-primary-color);
  color: var(--color-text-2);
}
.editor-content :deep(.tiptap pre) {
  padding: 10px 12px;
  overflow: auto;
  border-radius: 6px;
  background: var(--n-action-color);
}
.editor-content :deep(.tiptap img) {
  display: inline-block;
  max-width: min(100%, 560px);
  max-height: 360px;
  margin: 6px 0;
  border-radius: 6px;
  object-fit: contain;
  vertical-align: middle;
}
.editor-content :deep(.tiptap a) {
  color: var(--n-primary-color);
  text-decoration: underline;
}
.editor-content :deep(.tiptap ul),
.editor-content :deep(.tiptap ol) {
  padding-left: 1.6em;
}
</style>
