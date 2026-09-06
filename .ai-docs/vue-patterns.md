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

## Ark UI Integration

Ark UI adds no provider tree. Theme toggling is handled by `stores/theme.ts` (sets `data-theme` on `<html>`); design
color tokens live in `src/styles/global.css`. There is nothing to wrap app root with for theming.

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

---

## Unified Icon System (`AppIcon`)

All icons in the project go through a single `<AppIcon>` component that abstracts away the underlying icon source.

### Imports

```ts
// Direct component import (template usage)
import AppIcon from '@/components/AppIcon.vue'

// Or via composable (render function usage)
import { useIconRenderer } from '@/composables/use-icon'
```

### Template Usage

```vue
<!-- Iconify (default source) -->
<AppIcon icon="ri:home-3-line" />

<!-- LobeHub provider avatar -->
<AppIcon icon="openai" source="lobehub" />

<!-- Local image (本地图片) -->
<AppIcon icon="/src/assets/icon.png" source="ben" />

<!-- With custom size, radius, color, background -->
<AppIcon icon="ri:mail-line" :size="20" :color="#333" />
<AppIcon icon="ri:mail-line" background="#FF3333" />
<AppIcon icon="openai" source="lobehub" :size="24" :radius="6" />
```

### Render Function Usage (leading icon)

Return an icon node from a render-ish helper via `h(AppIcon, {...})` or a plain `h('span')`; no naive slot plumbing.

```ts
import { useIconRenderer } from '@/composables/use-icon'

const { renderIcon } = useIconRenderer()

const menuOptions = [
  { label: 'Home', key: 'home', icon: renderIcon('ri:home-3-line') },
  { label: 'OpenAI', key: 'openai', icon: renderIcon('openai', 'lobehub', 20) },
]
```

### v-bind with dynamic props

When the source type is not known at compile time (e.g., LLM provider tags), use `v-bind` with a props object:

```ts
import { getProviderIconConfig } from '@/composables/provider-icons'

function providerIconProps(provider: string, size = 14, radius = 3) {
  const cfg = getProviderIconConfig(provider)
  if (!cfg) return null
  return { ...cfg, size, radius }
}
```

```vue
<AppIcon v-if="providerIconProps(item)" v-bind="providerIconProps(item)!" />
```

### Available Sources

| `source` | Backend | Identifier format |
|---|---|---|
| `'iconify'` (default) | `@iconify/vue` | Iconify icon name (e.g. `'ri:home-3-line'`) |
| `'lobehub'` | `@lobehub/icons-static-avatar` | Provider key (e.g. `'openai'`, `'deepseek'`) |
| `'ben'` | Direct `<img>` src | File path or URL |

### Adding New Provider Icons

To add an icon for a new LLM provider:

1. Add a `@lobehub/icons-static-avatar` import and entry in `composables/provider-icons.ts` `ICON_MAP` for a static WebP avatar
2. Or add an iconify fallback in `FALLBACK_ICON_MAP` if no lobehub avatar exists

```ts
// In provider-icons.ts
const FALLBACK_ICON_MAP: Record<string, string> = {
  custom: 'streamline-color:brain',
  // Add new providers here:
  // 'my-provider': 'iconify-icon-name',
}
```
