<script setup lang="ts">
import { Editor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import { onBeforeUnmount, onMounted, shallowRef } from 'vue'

const editor = shallowRef<Editor>()

onMounted(() => {
  editor.value = new Editor({
    content: '',
    extensions: [StarterKit],
  })
})

onBeforeUnmount(() => {
  editor.value?.destroy()
})
</script>

<template>
  <div class="editor-wrapper">
    <EditorContent class="editor" :editor="editor" />
  </div>
</template>

<style scoped>
.editor-wrapper {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.editor {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.editor :deep(.ProseMirror) {
  flex: 1;
  padding: 16px;
  outline: none;
}
</style>
