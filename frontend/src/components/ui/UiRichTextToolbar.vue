<script setup lang="ts">
import { computed, onBeforeUnmount, ref, unref, watch } from 'vue'
import type { Editor } from '@tiptap/core'
import type { ShallowRef } from 'vue'
import { Dialog } from '@ark-ui/vue/dialog'
import UiColorPicker from './UiColorPicker.vue'
import UiDialog from './UiDialog.vue'

const props = defineProps<{
  editor: Editor | ShallowRef<Editor | undefined> | undefined
  textColorLabel: string
  backgroundColorLabel: string
  customColorLabel: string
  defaultColorLabel: string
  linkLabel: string
  linkPrompt: string
  cancelLabel: string
}>()

const revision = ref(0)
const linkOpen = ref(false)
const linkUrl = ref('')
let unsubscribe: (() => void) | null = null

const resolvedEditor = computed<Editor | undefined>(() => unref(props.editor))

watch(
  resolvedEditor,
  (editor) => {
    unsubscribe?.()
    unsubscribe = null
    revision.value++
    if (!editor || typeof editor.on !== 'function') return
    const onTransaction = () => {
      revision.value++
    }
    editor.on('transaction', onTransaction)
    unsubscribe = () => editor.off('transaction', onTransaction)
  },
  { immediate: true },
)

const textColor = computed(() => {
  void revision.value
  return resolvedEditor.value?.getAttributes('textStyle').color as string | undefined
})
const backgroundColor = computed(() => {
  void revision.value
  return resolvedEditor.value?.getAttributes('textStyle').backgroundColor as string | undefined
})
const linkActive = computed(() => {
  void revision.value
  return Boolean(resolvedEditor.value?.isActive('link'))
})

const textColorPresets = [
  '#111827',
  '#374151',
  '#dc2626',
  '#ea580c',
  '#ca8a04',
  '#16a34a',
  '#0891b2',
  '#2563eb',
  '#7c3aed',
  '#db2777',
]
const backgroundColorPresets = [
  '#ffffff',
  '#fef3c7',
  '#fee2e2',
  '#ffedd5',
  '#dcfce7',
  '#cffafe',
  '#dbeafe',
  '#ede9fe',
  '#fce7f3',
  '#e5e7eb',
]

function openLinkDialog() {
  linkUrl.value = (resolvedEditor.value?.getAttributes('link').href as string | undefined) ?? ''
  linkOpen.value = true
}

function applyLink() {
  const url = linkUrl.value.trim()
  if (!url) return
  resolvedEditor.value?.chain().focus().extendMarkRange('link').setLink({ href: url }).run()
  linkOpen.value = false
}

function setTextColor(color: string) {
  resolvedEditor.value?.chain().focus().setColor(color).run()
}

function unsetTextColor() {
  resolvedEditor.value?.chain().focus().unsetColor().run()
}

function setBackgroundColor(color: string) {
  resolvedEditor.value?.chain().focus().setBackgroundColor(color).run()
}

function unsetBackgroundColor() {
  resolvedEditor.value?.chain().focus().unsetBackgroundColor().run()
}

onBeforeUnmount(() => {
  unsubscribe?.()
})
</script>

<template>
  <span class="rich-text-inline-group">
    <UiColorPicker
      :label="textColorLabel"
      :custom-label="customColorLabel"
      :default-label="defaultColorLabel"
      icon="i-ri:font-color"
      :value="textColor"
      fallback="#dc2626"
      :presets="textColorPresets"
      @select="setTextColor"
      @reset="unsetTextColor"
    />
    <UiColorPicker
      :label="backgroundColorLabel"
      :custom-label="customColorLabel"
      :default-label="defaultColorLabel"
      icon="i-ri:mark-pen-line"
      :value="backgroundColor"
      fallback="#fef3c7"
      :presets="backgroundColorPresets"
      @select="setBackgroundColor"
      @reset="unsetBackgroundColor"
    />
    <button
      type="button"
      class="tb"
      :class="{ active: linkActive }"
      :title="linkLabel"
      @click="openLinkDialog"
    >
      <i class="i-ri:link" aria-hidden="true" />
    </button>
  </span>

  <UiDialog :open="linkOpen" size="narrow" @update:open="linkOpen = $event">
    <header class="dlg-head">
      <Dialog.Title class="dlg-title">{{ linkLabel }}</Dialog.Title>
      <Dialog.CloseTrigger
        type="button"
        class="dlg-close"
        :aria-label="cancelLabel"
        @click="linkOpen = false"
      >
        <i class="i-ri:close-line" aria-hidden="true" />
      </Dialog.CloseTrigger>
    </header>
    <form @submit.prevent="applyLink">
      <div class="dlg-body">
        <label class="link-field">
          <span>{{ linkPrompt }}</span>
          <input
            v-model="linkUrl"
            class="ctl"
            type="url"
            inputmode="url"
            placeholder="https://"
            autofocus
          />
        </label>
      </div>
      <footer class="dlg-foot">
        <button type="button" class="btn" @click="linkOpen = false">{{ cancelLabel }}</button>
        <button type="submit" class="btn btn--primary" :disabled="!linkUrl.trim()">
          {{ linkLabel }}
        </button>
      </footer>
    </form>
  </UiDialog>
</template>

<style scoped>
.rich-text-inline-group {
  display: inline-flex;
  align-items: center;
  gap: 2px;
}
.tb {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
  width: 28px;
  height: 28px;
  padding: 0;
  color: var(--color-text-2);
  background: transparent;
  border: 1px solid transparent;
  border-radius: 5px;
  cursor: pointer;
}
.tb:hover,
.tb.active {
  color: var(--n-text-color);
  background: var(--n-sidebar-hover);
}
.tb.active {
  color: var(--n-primary-color);
  border-color: color-mix(in srgb, var(--n-primary-color) 32%, transparent);
}
.tb i {
  font-size: 16px;
}
.link-field {
  display: grid;
  gap: 7px;
  color: var(--color-text-2);
  font-size: 12px;
}
</style>
