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
| Icons | `<NIcon>` with `@vicons/*` or raw SVG |

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

## Documentation

Refer to the official docs for component API details: https://www.naiveui.com/
