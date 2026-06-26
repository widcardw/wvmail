<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'

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
  // Inject sanitized HTML + dark mode overrides inside shadow DOM
  const darkStyles = props.dark
    ? `<style>
  :host * { background-color: transparent !important; border-color: rgba(128,128,128,0.25) !important; }
  :host(:where(p,div,span,li,td,th,h1,h2,h3,h4,h5,h6,blockquote,pre,section,article,ul,ol,dl)) { color: inherit !important; }
  :host a:not([style*=color]) { color: #66b0ff; text-decoration: underline; }
  </style>`
    : ''
  shadowRoot.innerHTML = darkStyles + props.html
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
