<script setup lang="ts">
import { computed } from 'vue'
import { JsonTreeView } from '@ark-ui/vue/json-tree-view'

const props = defineProps<{
  body: string
}>()

const parsedBody = computed<object | null>(() => {
  try {
    const value: unknown = JSON.parse(props.body)
    return value !== null && typeof value === 'object' ? (value as object) : null
  } catch {
    return null
  }
})
</script>

<template>
  <details v-if="props.body" class="response">
    <summary>Response body</summary>
    <JsonTreeView.Root
      v-if="parsedBody"
      :key="props.body"
      class="response-tree font-mono"
      :data="parsedBody"
      :default-expanded-depth="2"
      :expand-on-click="true"
      quotes-on-keys
    >
      <JsonTreeView.Tree class="response-tree__tree" indent-guide>
        <template #arrow>
          <span class="response-tree__arrow" aria-hidden="true">›</span>
        </template>
      </JsonTreeView.Tree>
    </JsonTreeView.Root>
    <pre v-else>{{ props.body }}</pre>
  </details>
</template>

<style scoped>
.response {
  margin-top: 6px;
}
.response summary {
  cursor: pointer;
  user-select: none;
}
.response pre {
  max-height: 180px;
  overflow: auto;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-word;
}
.response-tree {
  width: 100%;
  color: var(--n-text-color);
}
.response-tree :deep([data-part='branch-content']) {
  position: relative;
}
.response-tree :deep([data-part='branch-indent-guide']) {
  position: absolute;
  inset-inline-start: calc((var(--depth) - 1) * 1rem);
  width: 1px;
  height: 100%;
  background: var(--n-border-color);
}
.response-tree :deep([data-part='branch-indent-guide'][data-depth='1']) {
  inset-inline-start: 0.75rem;
}
.response-tree :deep([data-part='branch-control']) {
  display: flex;
  align-items: center;
  padding-inline-start: calc((var(--depth) - 1) * 0.75rem);
  user-select: none;
}
.response-tree :deep([data-part='branch-control'][data-depth='1']) {
  padding-inline-start: 0.25rem;
}
.response-tree :deep([data-part='branch-control']:hover),
.response-tree :deep([data-part='item']:hover) {
  background: var(--n-sidebar-hover);
}
.response-tree :deep([data-part='branch-indicator']) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin-inline-end: 0.25rem;
  transform-origin: center;
}
.response-tree :deep([data-part='branch-indicator'][data-state='open']) {
  transform: rotate(90deg);
}
.response-tree :deep([data-part='item']) {
  position: relative;
  display: flex;
  padding-inline-start: calc(((var(--depth) - 1) * 0.75rem) + 0.75rem);
}
.response-tree :deep([data-part='item'][data-depth='1']) {
  padding-inline-start: 1.5rem;
}
.response-tree :deep([data-part='item-text']),
.response-tree :deep([data-part='branch-text']) {
  display: flex;
  align-items: baseline;
}
.response-tree__arrow {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1rem;
  height: 1rem;
  color: var(--color-text-2);
  font-size: 16px;
  line-height: 1;
}
.response-tree__tree {
  display: flex;
  flex-direction: column;
  max-height: 340px;
  margin-top: 6px;
  overflow: auto;
  padding: 6px 0;
  font-size: 12px;
  line-height: 1.8;
}
.response-tree__tree :deep([data-kind='key']) {
  color: var(--n-primary-color);
}
.response-tree__tree :deep([data-type='string']) {
  color: #16a34a;
}
.response-tree__tree :deep([data-type='number']) {
  color: #d97706;
}
.response-tree__tree :deep([data-type='boolean']) {
  color: #2563eb;
}
.response-tree__tree :deep([data-type='null']),
.response-tree__tree :deep([data-type='undefined']) {
  color: var(--color-text-3);
}
</style>
