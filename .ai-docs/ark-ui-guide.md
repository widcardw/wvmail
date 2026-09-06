---
name: ark-ui-guide
description: Ark UI (headless) theming, import paths, styling with CSS Modules / UnoCSS / global tokens, and the naive-ui → ark-ui migration map for the wvmail project
type: reference
---

# Ark UI Guide

This project uses **Ark UI** (`@ark-ui/vue`, currently `^5.39.1`) for behavior and accessibility, replacing the former
**Naive UI** component library. **Ark UI is headless**: it ships no presentational styles and no baked-in theme, so
*every* visual concerns is ours to define. Whatever look we produce is up to us — this doc defines how we standardize it.

Reference mirror: a full dump of the Ark UI docs lives at `.ai-docs/arkui-vue.txt` (read sections per component while
editing). Official docs: https://ark-ui.com/

## Core mental model

- Ark components are **compound components**: import from a per-component path and nest `.Root`, `.Trigger`, `.Item`,
  etc. Each part carries `data-scope`/`data-part` (+ `data-state`, `data-focus`, …) attributes we target in CSS.
- Import style (do not import the whole barrel):
  ```ts
  import { Slider, createListCollection } from '@ark-ui/vue/slider'
  import { Dialog } from '@ark-ui/vue/dialog'
  import { createToaster, Toaster } from '@ark-ui/vue/toast'
  ```
- Everything (positioning, focus-trapping, flip, state, theming) is behaviors + hooks; **no `<NConfigProvider>`/forest
  providers exist**. There is no global `<ConfigProvider>` to wrap the app for theming. Locale/theme are our concern
  (see Theme below).
- Compounds like `<Slider.Root>.Root` composables (`useSlider`) exist for controlled/imperative use, but components are
  simpler.

## Import paths reference (naive → ark mappings)

Below maps what we currently name in code to the corresponding directory/module and headless anatomy. During migration,
consult the matching section in `arkui-vue.txt` before writing a component.

| Naive (removed) | Ark module to import from | Notes |
|---|---|---|
| NButton | `@ark-ui/vue/qr-code`? no — buttons are just `<button>`. We mostly use plain buttons + our `.btn`/globals | Ark exposes no bare Button; use native `<button>` styled by us |
| NInput / NTextarea | `@ark-ui/vue/field` / `field.textarea` or plain inputs; Ark `Input`/`PasswordInput` exist for some | Prefer semantic native controls + tokens unless we need password/prefix etc. |
| NSwitch | `@ark-ui/vue/switch` | `Switch.Root/.Control/.Thumb/.Label` |
| NCheckbox | `@ark-ui/vue/checkbox` | |
| NCollapse / NCollapseItem | `@ark-ui/vue/collapsible` | `Collapsible.Root/.Content/.Trigger` |
| NMenu | Sidebar account→folders → `@ark-ui/vue/tree-view`; utility/smart nav → plain list, not a menu. General popups only → `@ark-ui/vue/menu` | account folders expand lazily via TreeView `loadChildren`/`childrenCount`/`onLoadChildrenComplete`; folder selected via `selectionMode="single"` |
| NTooltip | `@ark-ui/vue/tooltip` | |
| NPopover / NPopconfirm | `@ark-ui/vue/popover` | confirm pattern via popover + buttons |
| NDropdown | `@ark-ui/vue/menu` | see Menu |
| NDialog / NModal | `@ark-ui/vue/dialog` | `Dialog.Root/.Content/.Backdrop…` unlocked by `open` state |
| NAlert | (no direct) | build small component or just styled block |
| NTag | none | styled `<span>` chips in our CSS |
| NCard | none | styled `<div>` blocks (`.surface`) |
| NEmpty | none | small inline empty-state block + icon |
| NSpin/Loading | none | CSS spinner/`<span class="spinner"/>` (or `Presence`) |
| NIcon / AppIcon | (see Icon section) | drop `NIcon` wrapper; `<AppIcon>` only |
| NAvatar/NAccountAvatar | `@ark-ui/vue/avatar` | not required for user pics |
| NList/NListItem | none / plain | semantic list via markup (or `Listbox`) |
| NPopconfirm | `popover` + native buttons | |
| NScrollbar | `@ark-ui/vue/scroll-area` or CSS `overflow` | custom scroll area optional; CSS first |
| NForm/NFormItem/NFormItemGi | `@ark-ui/vue/fieldset` / reactive-forms | see Forms section; often plain grid + label is enough |
| NSelect | `@ark-ui/vue/select` | anatomy with `createListCollection` |
| NInputNumber | `@ark-ui/vue/number-input` | |
| NColorPicker | `@ark-ui/vue/color-picker` | |
| NGrid/NGridItem | none | use plain CSS grid utilities |
| NSpace/NFlex/NLayout/NLayoutSider... | none | plain CSS (flex/grid) — remove |
| NSplit | `@ark-ui/vue/splitter` | vertical split of mail list/detail |
| NCalendar | `@ark-ui/vue/date-picker` only if needed | reminders currently `NCard`+`NEmpty` |
| NCode | none | syntax block with actual code elements |
| NUpload | `@ark-ui/vue/file-upload` | account/avatar upload |
| useMessage/useDialog (naive) | **no equivalent inject.** Use `@ark-ui/vue/toast` `toaster`, or simple `ref` booleans / our message component | see Feedback below |
| useLoadingBar | CSS | n/a |

_(Remove layout wrappers NCard/NLayout/NSpace/NFlex entirely — replace with our own grid/flex CSS.)_

## Required style per component

Each component/part gets its styles in one of these (pick lowest-friction):

1. **Global CSS util → UnoCSS.** Everything shared and token-derived (`.btn`, focus ring, paddings) declared once using
   design tokens so light/dark both work.
2. **SFC `<style scoped>`** for self-contained cases that only restyle tokens / data-scope–specific parts.
3. **CSS Modules (`*.module.css`)** for Ark compound parts we reuse across SFCs, keyed by our own high-level classes.
   Ark target via `:global([data-part])` inside a module only when needed to decouple.

We try NOT to write near-identical 12-part re-styles per Ak component in one file. Prefer semantic helpers.

## Theme & design tokens (global.css)

**There is no component-forest theme provider.** Toggle handled by `src/stores/theme.ts`:

- mode Light / Dark / System.
- `data-theme` toggled on `<html>` → all tokens below flip automatically. Theme styles therefore never import theme
  logic; they only consume variables.
- primary color token `--color-primary` is configurable at runtime via existing `theme.setPrimaryColor`.

Naive used to publish `useThemeVars()` and `--n-*`. All such are removed. Reference tokens, never literal hex for the
surface palette:
- `--color-bg`, `--color-bg-subtle`, `--color-surface`, `--color-border`, `--color-text`, `--color-text-2`,
  `--color-text-3`, `--color-primary`, hover/pressed variants, `--color-hover-bg`, shadows, radii, font sizes.
Again: read the final list in `src/styles/global.css` — doc should not drift from source of truth.

By default components are totally unstyled: after swapping imports you must add styles or a control/button may look
bare. Do not rely on training defaults; target Ark parts by `data-scope`/`data-part`.

## Forms

Remove `NForm/NFormItem/NSelect` large APIs. Sections like settings that are straightforward can be plain markup:
`<label>` + control + CSS grid (`label-width` col 140, as before). If you need validation behavior, use
`@ark-ui/vue/fieldset` + `createForm` (ark “Formisch” style) or keep to manual. Prefer shared tokens for label width.

## Feedback / confirmation

- ✓ Lightweight success/error display: our small toast util on `@ark-ui/vue/toast`: build once, reuse everywhere.
- Confirmation: Dialog (`.Root` with open) or popover; replace `NPopconfirm`.

## Icons

The whole icon surface funnels through **`<AppIcon>`** (`src/components/AppIcon.vue`), not raw icon tags.
- Iconify icons are **offline UnoCSS classes** (`i-ri:sun-line`) from `@iconify-json/*` + `presetIcons`; `<AppIcon>` renders
  an inner `<i :class="`i-${icon}`">` glyph. We do NOT use `@iconify/vue` runtime `<Icon>` (no CDN fetch) and no naive `NIcon`.
- Installed offline sets (as devDeps): `ri`, `material-symbols`, `eos-icons`, `streamline-color`, `logos`, `vscode-icons`
  (add `@iconify-json/<set>` if a needed prefix is missing).
- `lobehub` provider avatars / `ben` local images render as `<img>` (unchanged).
- **Dynamic icons** (runtime-chosen: attachment by mime, provider fallbacks) are driven by static maps but only referenced as
  strings at runtime — so they must also be published. Any icon name not a literal `i-*` token in source is listed in the
  UnoCSS **`safelist`** (`uno.config.ts`), which forces emission. When you add a runtime/map icon, add its class there too.
- Icon helper VNodes (`renderProviderIcon`, `ICON_MAPPING[*].create`) return `h('i', {class:'i-…'})` or `<img>`; no naive wrapper.

## Mail sidebar = TreeView (accounts → folders)

The account/folder sidebar is an expandable **tree**, not a menu. Replace the three stacked naive `NMenu`s with:
- smart folders & utility nav: simple rows (not menus) — plain buttons + `router`;
- account→folders: **`TreeView`** (single root per sidebar) — each account is a lazy-loadable **branch** whose children
  are folder leaves, using Ark's Lazy Loading contract:

```vue
<script setup lang="ts">
import { type TreeCollection, TreeView, createTreeCollection } from '@ark-ui/vue/tree-view'
import { ref } from 'vue'
import type { EmailAccount } from '~/types/email'

interface SideNode { id: string; label: string; kind: 'account' | 'folder' | 'loading'; icon?: string }

// accounts come from the store; folders pulled per-account when its branch first expands:
const collection = ref<TreeCollection<SideNode>>() // seed root children = [account branches…]

function onSelected(details: { value: string[] }) {
  // details.value = selected folder '<accountId>:<folder>'; route to that folder
}
</script>

<template>
  <TreeView.Root
    :collection="collection"
    selection-mode="single"
    expand-on-click
    v-model:selected-value="selectedValue"
    :load-children="loadAccountFolders"
    @load-children-complete="(e) => (collection = e.collection)"
  >
    <TreeView.Tree>
      <SideBarNode v-for="(node, i) in collection.rootNode.children" :key="node.id" :node="node" :index-path="[i]" />
    </TreeView.Tree>
  </TreeView.Root>
</template>
```

- Branch = account (collapsible, `AccountAvatar` icon + name); leaves = folders (their categorized `i-*` icon +
  localized label). An emitting account with kids not loaded yet renders only a branch carrying
  `childrenCount: folders.length` and Ark lazy-loaded them via `loadChildren` (wraps
  `accountsStore.loadFolders`); a loading account shows its branch with default loading node or the store's loading
  branch text until `onLoadChildrenComplete` swaps in the fresh collection (from the store, as in the Ark Lazy Loading
  example in `arkui-vue.txt`).
- `collapsed` param (app rail collapsed) is a separate CSS concern in `App.vue`/`TitleBar`, not prop plumbing into
  TreeView; reflow row widths via CSS once collapsed.

## Story-driving note for migration (current progress)

**Full tail is migrated off `naive-ui`.** No source imports `naive-ui` or `@iconify/vue` anymore; the temporary
`naive-ui` dependency is removed; router has the real screens live:
- `/` → `MailView` (EmailList + EmailDetail naive-free, flex list/detail split)
- `/notes`, `/reminder`, `/settings/*` (SettingsMain + Common/Account/About/LLM/Translation)
- account→folders sidebar is Ark **TreeView** (lazy-loads folders per account on first expand, see above)
Use global design tokens in `src/styles/global.css`; icons are offline `i-*` classes (see icon section + `uno.config.ts`
safelist). Transient messages via `stores/feedback.ts` + `<ToastHost/>`; confirmations use two-step buttons /
`window.confirm`. Ark per-component `*.module.css` styling is optional on top of `scoped`, per component UI.

Possible residual: verify interaction in `wails3 dev`, and confirm the rich-email render (EmailShadowBody + themed email
sanitize) matches the old look; if needed, polish individual part CSS. The temp naive remnant was deleted; keep the repo
build-green with `pnpm --dir frontend run build:dev`.
