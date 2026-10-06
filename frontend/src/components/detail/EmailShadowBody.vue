<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { adaptHtmlForDarkMode } from '~/composables/html'

const props = defineProps<{
  html: string
  dark?: boolean
}>()

const container = ref<HTMLElement | null>(null)
let shadowRoot: ShadowRoot | null = null

function updateContent() {
  if (!container.value) return
  if (!shadowRoot) {
    shadowRoot = container.value.attachShadow({ mode: 'open' })
  }

  let content = props.html
  if (props.dark) {
    try {
      content = adaptHtmlForDarkMode(content)
    } catch {
      // Dark-mode rewriting is best-effort: if it throws, render the original
      // HTML instead of leaving the body blank or showing stale content.
      content = props.html
    }
  }

  shadowRoot.innerHTML = content
}

onMounted(updateContent)
watch(() => [props.html, props.dark], updateContent)
</script>

<template>
  <div ref="container" class="shadow-body"></div>
</template>

<style scoped>
.shadow-body {
  line-height: 1.6;
  font-size: 14px;
  word-break: break-word;
}
</style>
