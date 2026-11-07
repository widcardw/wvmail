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
import { useSomeStore } from '~/stores/someStore'

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

Scalar settings that mirror one `config.yaml` field use
`persistedSetting(ref, field, failureMessage)` in `stores/preferences.ts`. It owns
the optimistic assignment, string conversion and shared error logging; add a new
boolean/number/string preference by composing it instead of copying a setter.

## Ark UI Integration

Ark UI adds no provider tree. Theme toggling is handled by `stores/theme.ts` (sets `data-theme` on `<html>`); design
color tokens live in `src/styles/global.css`. There is nothing to wrap app root with for theming.

## Wails Bridge

- Go services are imported from `#/wvmail/services` (the `#` alias points at
  the generated `bindings/` directory)
- Events from Go are listened to via `Events.On()` from `@wailsio/runtime`
- Use the `~` alias for `src/` imports (configured in `vite.config.js`)

## Path Aliases

- `~/` maps to `frontend/src` — use for all internal imports (e.g.
  `import { useThemeStore } from '~/stores/theme'`)
- `#/` maps to `frontend/bindings` — use for generated Wails service and model
  imports (e.g. `import { EmailService } from '#/wvmail/services'`)
- There is no `@/` alias in this project.

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
| `pnpm run typecheck` | Type-check `.vue` and `.ts` with `vue-tsc` |
| `pnpm run test` | Run the Node regression tests in `tests/*.test.mts` |
| `pnpm run fmt` | Auto-format code (oxfmt) |

---

## Unified Icon System (`AppIcon`)

All icons in the project go through a single `<AppIcon>` component that abstracts away the underlying icon source.

### Imports

```ts
// Direct component import (template usage)
import AppIcon from '~/components/icon/AppIcon.vue'

// For a render function, build the node from the provider helpers instead:
import { renderProviderIcon, renderProviderOption } from '~/composables/icons/providers'
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

Select-like components take an `icon` field holding a VNode, so build it with the provider
helpers (`composables/icons/providers.ts`) rather than with `h(AppIcon, …)`:

```ts
import { renderProviderIcon } from '~/composables/icons/providers'

const providerOptions = providers.map((p) => ({
  label: p.label,
  value: p.key,
  icon: renderProviderIcon(p.key, 16), // provider avatar, or its iconify fallback
}))
```

Generic (non-provider) icon nodes come from `icon-mapping`'s `brandCreate`/`ICON_MAPPING`, which
is also what `AccountAvatar` uses for mail providers.

### v-bind with dynamic props

When the source type is not known at compile time (e.g., LLM provider tags), use `v-bind` with a props object:

```ts
import { getProviderIconConfig } from '~/composables/icons/providers'

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
| `'iconify'` (default) | offline UnoCSS preset icons (`.i-<name>`, no runtime fetch) | Iconify icon name (e.g. `'ri:home-3-line'`) |
| `'lobehub'` | `@lobehub/icons-static-avatar` | Provider key (e.g. `'openai'`, `'deepseek'`) |
| `'ben'` | Direct `<img>` src | File path or URL |

### Adding New Provider Icons

To add an icon for a new LLM provider:

1. Add a `@lobehub/icons-static-avatar` import and entry in `composables/icons/providers.ts` `ICON_MAP` for a static WebP avatar
2. Or add an iconify fallback in `FALLBACK_ICON_MAP` if no lobehub avatar exists

```ts
// In composables/icons/providers.ts
const FALLBACK_ICON_MAP: Record<string, string> = {
  custom: 'streamline-color:brain',
  // Add new providers here:
  // 'my-provider': 'iconify-icon-name',
}
```
