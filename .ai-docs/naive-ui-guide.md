---
name: naive-ui-guide
description: Naive UI component library usage patterns for the wvmail project
type: reference
---

# Naive UI Guide

This project uses **Naive UI** (`^2.44.1`) as the component library. Naive UI provides a comprehensive set of Vue 3 components with built-in dark mode support.

## Imports

Import components directly from `naive-ui` — tree-shaking is handled by Vite:

```ts
import { NButton, NInput, NConfigProvider, darkTheme, useOsTheme } from 'naive-ui'
```

## Theme System

The app is wrapped in `<NConfigProvider>` in `App.vue`. Theme toggling is managed by the Pinia store at `src/stores/theme.ts`.

```vue
<NConfigProvider :theme="theme.naiveTheme">
  <NButton @click="theme.switchMode()">Theme {{ theme.mode }}</NButton>
</NConfigProvider>
```

- `theme.mode` → light/dark/system
- `theme.naiveTheme` → Naive UI theme injected into `NConfigProvider`
- OS theme is auto-detected via `useOsTheme()` as the initial value

## Common Components

Use Naive UI components instead of raw HTML elements for all UI:

| Purpose | Naive UI Component |
|---|---|
| Buttons | `<NButton>` |
| Text Input | `<NInput>` |
| Forms | `<NForm>`, `<NFormItem>` |
| Layout | `<NSpace>`, `<NFlex>`, `<NLayout>`, `<NCard>` |
| Data Table | `<NDataTable>` |
| Menu/Sidebar | `<NMenu>` |
| Notifications | useMessage() from `naive-ui` |
| Dialog/Modal | useDialog() or `<NModal>` |
| Icons | `<NIcon>` with `@iconify/vue` — use `<AppIcon>` wrapper for unified icon sources |

## Component Size Variants

Most components support `size` prop: `'small' | 'medium' | 'large'`

## API Hooks

Naive UI provides inject-style hooks (must be called inside component setup):

```ts
import { useMessage, useDialog, useNotification, useLoadingBar } from 'naive-ui'

const message = useMessage()
message.success('Done!')
message.error('Failed!')
```

## Custom Theme Variables

To customize Naive UI theme beyond dark/light, pass theme overrides:

```vue
<NConfigProvider :theme-overrides="themeOverrides">
  <!-- content -->
</NConfigProvider>
```

## TypeScript

All Naive UI components ship TypeScript types. Use `GlobalTheme` type for theme references:

```ts
import type { GlobalTheme } from 'naive-ui'
```

## Icons in Naive UI

This project uses **`<AppIcon>`** (not raw `@iconify/vue` `<Icon>`) for all icons. The component automatically wraps iconify icons in `<NIcon>` and handles lobehub/ben sources as `<img>`.

```vue
<!-- ✅ Correct — use AppIcon everywhere -->
<AppIcon icon="ri:home-3-line" />

<!-- Inside Naive UI icon slots -->
<NButton>
  <template #icon>
    <AppIcon icon="ri:settings-2-line" />
  </template>
</NButton>
```

For render functions (NMenu options, etc.), use the `useIconRenderer()` composable:

```ts
import { useIconRenderer } from '@/composables/use-icon'

const { renderIcon } = useIconRenderer()
const menuOption = {
  label: 'Home',
  key: 'home',
  icon: renderIcon('ri:home-3-line'),
}
```

For icons whose source is unknown at compile time (e.g. LLM provider tags), use `v-bind`:

```vue
<AppIcon v-if="providerIconProps(item)" v-bind="providerIconProps(item)!" />
```

See `vue-patterns.md` for the full AppIcon API reference.

## Documentation

Refer to the official docs for component API details: https://www.naiveui.com/
