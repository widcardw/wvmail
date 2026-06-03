---
name: vue-patterns
description: Vue 3 Composition API patterns and conventions used in the wvmail project
type: reference
---

# Vue 3 Patterns

This project uses **Vue 3** with the **Composition API** and **TypeScript**.

## Component Structure

All components use `<script setup lang="ts">`:

```vue
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useSomeStore } from '@/stores/someStore'

const props = defineProps<{ title: string }>()
const emit = defineEmits<{ change: [value: string] }>()

const count = ref(0)
const doubled = computed(() => count.value * 2)

onMounted(() => { /* init logic */ })
</script>

<template>
  <div>{{ props.title }} — {{ doubled }}</div>
</template>
```

## State Management (Pinia)

Stores go in `src/stores/` and use the Options API or Setup API style:

```ts
// src/stores/example.ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useExampleStore = defineStore('example', () => {
  const count = ref(0)
  const doubled = computed(() => count.value * 2)
  function increment() { count.value++ }

  return { count, doubled, increment }
})
```

## Naive UI Integration

Wrap app root with `<NConfigProvider>` for theming. Access theme store for dark/light toggle.

## Wails Bridge

- Go services are imported from `../../bindings/wvmail` (auto-generated)
- Events from Go are listened to via `Events.On()` from `@wailsio/runtime`
- Use `@` alias for `src/` imports (configured in vite.config.js)

## Path Aliases

- `@/` maps to `src/` — use for all internal imports (e.g., `import { useThemeStore } from '@/stores/theme'`)

## Code Style

- No semicolons, single quotes (oxfmt)
- TypeScript strict mode
- Linting via oxlint
- Ignore `bindings/`, `dist/`, `node_modules/` for linting/formatting

## Lint / Format Commands

Run from the `frontend/` directory:

| Command | Purpose |
|---|---|
| `pnpm run lint` | Check for lint errors (oxlint) |
| `pnpm run fmt` | Auto-format code (oxfmt) |
