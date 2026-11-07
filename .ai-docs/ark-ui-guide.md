---
name: ark-ui-guide
description: Ark UI (headless) component choices, shared wrappers, theming, import paths, and CSS conventions in the wvmail project
type: reference
---

# Ark UI Guide

This project uses **Ark UI** (`@ark-ui/vue`, currently `^5.39.1`) for behavior
and accessibility. **Ark UI is headless**: it ships no presentational styles and
no baked-in theme, so *every* visual concern is ours to define. This doc defines
the current component choices and how we standardize the visuals.

Reference mirror: a full dump of the upstream Ark UI docs lives at
`.ai-docs/reference/arkui-vue.txt`. It may lag the installed package; official
docs: https://ark-ui.com/

## Core mental model

- Ark components are **compound components**: import from a per-component path and nest `.Root`, `.Trigger`, `.Item`,
  etc. Each part carries `data-scope`/`data-part` (+ `data-state`, `data-focus`, …) attributes we target in CSS.
- Import style (do not import the whole barrel):
  ```ts
  import { Slider, createListCollection } from '@ark-ui/vue/slider'
  import { Dialog } from '@ark-ui/vue/dialog'
  import { createToaster, Toaster } from '@ark-ui/vue/toast'
  ```
- Everything (positioning, focus-trapping, flip, state, theming) is behaviors + hooks; there is no component-forest
  provider to wrap the app with. Locale/theme are our concern
  (see Theme below).
- Compounds like `<Slider.Root>.Root` composables (`useSlider`) exist for controlled/imperative use, but components are
  simpler.

## Current component choices

Use the narrowest primitive that fits the job. Prefer native semantic elements
and the shared wrappers before reaching for a new Ark compound.

| Need | Use | Notes |
|---|---|---|
| Buttons | native `<button>` + `.btn` / `.icon-btn` | Ark has no bare Button; the shared classes own the look |
| Text input / textarea | Ark `Field.Input` / `Field.Textarea` | A raw `<input>` does not register as the field control |
| Switch | `components/ui/UiSwitch.vue` | `size="sm"` is available for dense toolbars |
| Checkbox | `components/ui/UiCheckbox.vue` | Use `.checkbox-row` in settings grids |
| Select | `components/ui/UiSelect.vue` | Native select styling where a full listbox is unnecessary |
| Number input | `components/ui/UiNumberInput.vue` | Controlled; emits `value` |
| Password input | `components/ui/UiPasswordInput.vue` | Keeps the native `value` + `@input` contract |
| Color picker | `components/ui/UiColorPicker.vue` | Toolbar and field variants |
| Collapsible content | `@ark-ui/vue/collapsible` | `Collapsible.Root/.Content/.Trigger` |
| Popup menu | `@ark-ui/vue/menu` | Sender actions, context menus and account menus |
| Tooltip | `@ark-ui/vue/tooltip` | Short labels only |
| Dialog | `components/ui/UiDialog.vue` + `UiDialogHeader.vue` | Shared `.dlg-*` shell; pass `lazy-mount` + `unmount-on-exit` |
| Confirmation | `components/ui/ConfirmDialog.vue` | Do not use `window.confirm` |
| Scroll area | `components/ui/UiScrollArea.vue` | Shared narrow scrollbar; CSS overflow first for simple panes |
| Toast | `stores/feedback.ts` + `ToastHost.vue` | One app-level toaster |
| Inline notice | `components/ui/UiAlert.vue` | Shared info/success/warning/error styling |
| Account/folder sidebar | `@ark-ui/vue/tree-view` | Lazy-load folders per account |
| Mail list/detail split | `mail-split.module.css` | Custom seam; see the Splitter trap below |
| Date/time formatting | `composables/formats.ts` | `Format.Time` defaults to 24h and ignores the locale hour cycle |
| Byte formatting | `@ark-ui/vue/format` `Format.Byte` | Used for attachment sizes |

### Shared primitives — use these, don't hand-roll

Ark ships no visuals, so a few thin wrappers own the app's look. Reach for them instead of writing
another one-off (the settings area used to have three hand-rolled switches plus a native checkbox):

| Primitive | Wraps | Visuals live in |
|---|---|---|
| `components/ui/UiSwitch.vue` | `Switch.Root/.Label/.Control/.Thumb/.HiddenInput` | `styles/ui.css` (`.switch`, `.switch__thumb`, driven by `[data-state='checked']`) |
| `components/ui/UiCheckbox.vue` | `Checkbox.Root/.Control/.Indicator/.Label/.HiddenInput` | `styles/ui.css` (`.checkbox`, `.checkbox__indicator`) |
| `components/ui/UiSelect.vue` | native `<select>` styling | component |
| `components/ui/UiNumberInput.vue` | `NumberInput.Root/.Control/.Input/.IncrementTrigger/.DecrementTrigger` | `styles/ui.css` (`.num*`); the **control** is the visible box, so a caller `class` (`.input--sm`, `.numsel`) sizes the box and the steppers stay in flow next to the input |
| `components/ui/UiPasswordInput.vue` | `PasswordInput.Root/.Control/.Input/.VisibilityTrigger` | `styles/ui.css` (`.pw*`; the two eye icons swap on `[data-state='visible']`) |
| `components/ui/UiColorPicker.vue` | Ark `Popover` + preset swatches + native custom color input | `variant="toolbar"` for the editor icon, `variant="field"` for a select-like form control; the positioner needs an explicit high z-index inside modal dialogs because Ark's nested `--z-index` can otherwise put it behind the dialog card |
| `components/ui/BusyButton.vue` | plain `<button>` + busy/秒数 label | component |
| `components/ui/UiBadge.vue` | **nothing — Ark has no Badge**（`@ark-ui/vue` 里没有 `badge` 包，已核对安装的组件目录） | `styles/ui.css`（`.badge`，主色药丸）。props：`count`（≤0 直接不渲染，所以 `v-if` 收在组件里）、`max`（默认 99 → `99+`）、`label`（`aria-label`/tooltip 带**真实**数字，因为可见文字被缩写）。格式化是纯函数 `composables/badge.ts` 的 `formatBadgeCount`，`tests/badge.test.mts` 覆盖 |
| `components/ui/UiScrollArea.vue` | `ScrollArea.Root/.Viewport/.Content/.Scrollbar/.Thumb/.Corner` | component；5px thumb 只在内容溢出且指针悬停、滚动或拖动时显示，不因焦点残留而常驻 |
| `components/ui/UiAlert.vue` | **nothing — Ark has no inline alert** | shared `info/success/warning/error` inline notices with optional dismissal and icon override; use it instead of page-specific alert boxes |
| `components/ToastHost.vue` | `@ark-ui/vue/toast` `Toaster`/`Toast.Root` | card in the component, Ark's required root styling in `styles/ui.css`, driven by `stores/feedback.ts` |
| `components/ui/UiDialog.vue` | `Dialog.Root/.Backdrop/.Positioner/.Content` | shared shell: `.dlg-back`, `.dlg-pos(--top)`, `.dlg-card(--narrow/--wide/--padded)`, `.dlg-body`, `.dlg-foot`, `.dlg-actions`. Cards are flush by default; pass `padded` only for compact dialogs without the standard head/body/foot. Always pass **`lazy-mount` + `unmount-on-exit`** |
| `components/ui/UiDialogHeader.vue` | `Dialog.Title/.CloseTrigger` | standard `.dlg-head` / `.dlg-title` / `.dlg-close` row. Use it for every normal dialog header; only specialised headers (confirm, crop) compose Ark directly |

Dialogs that can trigger a remote request or external side effect pass
`close-on-interact-outside="false"`. This includes the LLM editor, AI tag
suggestions, the external-link confirmation, and remote mail deletion. Backdrop
clicks must not discard a reviewed prompt or an in-flight action; explicit close,
cancel, Escape, or a deliberate confirmation remains available.

`UiSwitch` usage: pass the label through the default slot and `label-class` (the Ark root is a
`<label>`, so the whole row toggles); omit the slot when the label must sit in a row that also holds
other controls. Bind with `:checked` + `@update:checked="…"`. Use `size="sm"` for compact
toolbar/header rows; the switch keeps the same focus and disabled recipes as the default size.

`UiCheckbox` follows the same slot contract. For settings grids, opt into `.checkbox-row` +
`label-class="checkbox-row__label"`: the root then supplies one consistent clickable row with
vertical centering, an 8px control/text gap and hover/disabled states. Do not wrap it in another
`<label>`; the Ark root is already the label.

Shared form sizing lives in `styles/ui.css`: `.ctl` is block-width by default, and
`.dlg-body .input` makes ordinary inputs fill the dialog body. Do not re-declare
`width: 100%` in each dialog/component.

**Trap — `Splitter` + overlays.** Both splitters used `Splitter.createRegistry()` (its
`hitAreaMargins` make a 1px seam easy to grab). The registry hit-tests every `pointerdown` from a
*document-capture* listener and is meant to ignore handles that sit under a higher element, but that
check calls `getComputedStyle()` on the `document` node, throws, and is swallowed — so an open modal
dialog could not stop a drag (`RemindFromMailDialog`: the sidebar seam stayed draggable behind the
backdrop). `mail-split.module.css` now draws the seam with `::before` inside a 5px transparent
trigger (2px negative margin each side ⇒ identical layout) and no registry is passed, so the drag
starts from a real pointerdown that the `.dlg-back` overlay blocks. Its center knob is hidden until
hover/focus/resize so it does not read as a stray scrollbar.

**Disabled state has one recipe** (`styles/ui.css`, "Disabled controls"): the fill drops to
`--n-body-color`, the text to `--color-text-3`, and the cursor becomes `not-allowed`. Ark gives us
both hooks — `data-disabled` on the part it owns (`.num`, `.pw`, `.switch`, `.checkbox`,
`.u-trigger`) and a real `disabled` on inputs — and `Field` marks its label/helper text, so a whole
row dims together. A new wrapper must adopt it, otherwise its disabled state looks editable.

`UiNumberInput` / `UiPasswordInput` contracts differ on purpose: NumberInput is **controlled** (Ark emits
`value-change`, so the wrapper emits `value: [number]` — no DOM event), PasswordInput keeps the native
`value` + `@input="(event: Event)"` contract so it drops into existing handlers.

## Required style per component

Each component/part gets its styles in one of these (pick lowest-friction):

1. **Global CSS util → UnoCSS.** Everything shared and token-derived (`.btn`, focus ring, paddings) declared once using
   design tokens so light/dark both work.
2. **SFC `<style scoped>`** for self-contained cases that only restyle tokens / data-scope–specific parts.
3. **CSS Modules (`*.module.css`)** for Ark compound parts we reuse across SFCs, keyed by our own high-level classes.
   Ark target via `:global([data-part])` inside a module only when needed to decouple.

We try NOT to write near-identical 12-part re-styles per Ak component in one file. Prefer semantic helpers.

### Visually-hidden parts must have a positioned ancestor

Ark ships some parts with an **inline** style, not just classes — the ones that matter here are the hidden inputs:
`Switch.HiddenInput`, `Checkbox.HiddenInput` and `SegmentGroup.ItemHiddenInput` render a 1px `<input>` with

```
position: absolute; width: 1px; height: 1px; margin: -1px;
clip: rect(0 0 0 0); overflow: hidden; border: 0; padding: 0;
```

Note there is **no `top`/`left`**: the box is placed at its *static position*, and its containing block is the nearest
positioned ancestor — or the initial containing block if there is none. Without one it ends up at the **end of the
document**, past the app's own layout, which silently extends the document's scrollable height. The damage only shows
up on the first focus: clicking the switch makes the browser scroll the document to reveal that 1px box, so the whole
window jumps up by that distance. `overflow: hidden` on `html/body` does **not** undo it — the wheel cannot scroll
back, and the user is left staring at a blank strip under the app.

`styles/ui.css` therefore makes the roots positioned and pins the inputs:

```css
[data-scope='switch'][data-part='root'],
[data-scope='checkbox'][data-part='root'],
[data-scope='segment-group'][data-part='item'] { position: relative; }

[data-scope='switch'][data-part='hidden-input'],
[data-scope='checkbox'][data-part='hidden-input'] { top: 0; left: 0; }
```

SegmentGroup's input is one level deeper — inside `Item`, which is a `<label>` — so there the *item* is the ancestor
that has to be positioned (its input has no `data-part` attribute, so it cannot be targeted directly).

Any new Ark component that renders a hidden input (RadioGroup, Toggle, PinInput, Select's `HiddenSelect`…) needs the
same two rules. To check: in devtools, `document.scrollingElement.scrollHeight` must equal `window.innerHeight` on a
page that does not scroll — if it is larger, something is sitting outside the layout.

### Indicators need `orientation` **and** an explicit width

`Tabs` and `SegmentGroup` both expose an `Indicator` that Ark positions from CSS variables measured off the selected
item. Two things are easy to get wrong, and both fail *silently*:

- **`orientation` has no default.** zag writes `left: var(--left)` only when the prop is literally `"horizontal"`
  (`[prop("orientation") === "horizontal" ? "left" : "top"]: …`). Omit it and the indicator gets `top: var(--top)`
  instead: it is pinned to the top of the strip, keeps its *static* left position and never follows the selection.
  Always pass `orientation="horizontal"` on a horizontal strip (it also sets `aria-orientation`).
- **Ark does not set the used width.** It publishes `--width`, but no `width`. An absolutely positioned element with
  only `left` set is shrink-to-fit, so an empty indicator renders **zero-width and invisible**. Write
  `width: var(--width)` yourself.

The reminders page lived with both bugs (an invisible 2px bar at the top of the tab strip); `AccountForm.vue`'s
`.head-tabs__indicator` and `ReminderView.vue`'s `.tabs__indicator` are the reference. Since Ark also writes
`transition-property/duration` inline, a `transition` declared in CSS loses — leave the animation to Ark and set
`--transition-duration: 0ms` on a headless harness if you need to measure the settled position.

### Tabs vs SegmentGroup: pick by where the panes live

`Tabs.Content` reads the tab machine from **context**, so it only renders as a descendant of `Tabs.Root`. That is fine
when the strip and the panes are in the same region (`ReminderView.vue`: list on top, content below). It is the wrong
tool when they are not — the account form's strip sits in the card header while its two panes sit in the card body,
with the avatar row and sync section shared by both. Making Tabs work there would mean wrapping the whole card in
`Tabs.Root`, i.e. handing the card's markup to the tab machine for a two-item switch.

`SegmentGroup` is a **radio group**: it owns only the value, and the panes stay plain `v-if` branches where they are.
Use it for a compact "pick one mode" control; keep `Tabs` when the selection really swaps a panel.

- `SegmentGroup.Item` is a `<label>` with a hidden radio, so **state lives in `data-state="checked"`** (not
  `data-selected`), and the focus ring has to be drawn on the item: `[data-focus-visible] { outline: … }`.
- Its `update:modelValue` can emit `null`; guard it (`if (value) tab.value = value`) instead of widening the ref.
- The group needs an accessible name — `SegmentGroup.Label` renders a `<span>` with the id that the root's
  `aria-labelledby` points at, so a visually-hidden label (`.vh`) is enough. Without it the reference dangles.
- `SegmentGroup.ItemHiddenInput` is required for the radio semantics; without it the item is a label pointing nowhere.

## Theme & design tokens (global.css)

**There is no component-forest theme provider.** Toggle handled by `src/stores/theme.ts`:

- mode Light / Dark / System.
- System mode stores `matchMedia('(prefers-color-scheme: dark)').matches` in a reactive ref and updates it from the
  query's `change` event. Do not call `matchMedia` inside the computed itself; it is not a Vue dependency, so the
  resolved theme would remain stuck at its initial value.
- `data-theme` toggled on `<html>` → all tokens below flip automatically. Theme styles therefore never import theme
  logic; they only consume variables.
- primary color token `--color-primary` is configurable at runtime via existing `theme.setPrimaryColor`.

Reference tokens, never literal hex for the surface palette:
- `--color-bg`, `--color-bg-subtle`, `--color-surface`, `--color-border`, `--color-text`, `--color-text-2`,
  `--color-text-3`, `--color-primary`, hover/pressed variants, `--color-hover-bg`, shadows, radii, font sizes.
Again: read the final list in `src/styles/global.css` — doc should not drift from source of truth.

By default Ark components are totally unstyled: a new control needs explicit styles or it may look bare. Do not rely
on training defaults; target Ark parts by `data-scope`/`data-part`.

## Forms

Every labelled control goes through Ark `Field`:

```vue
<Field.Root class="row">                      <!-- or class="frow" / class="field" -->
  <Field.Label class="row__label">Host</Field.Label>
  <Field.Input class="input" :model-value="form.Host" @update:model-value="form.Host = String($event)" />
  <Field.HelperText class="field__hint">Optional hint</Field.HelperText>
</Field.Root>
```

- **Why**: `Field.Label` renders a `<label for>` pointing at the field's control id, so the association is real
  (no more sibling `<label>` + `<input>` pairs that only *look* connected).
- **Ark form components link themselves.** Docs, `Field`: “Form components in Ark UI automatically integrate with `Field`
  through context.” So `UiSelect` / `UiNumberInput` / `UiPasswordInput` / `UiSwitch` / `UiCheckbox` need no id wiring —
  just place them inside `Field.Root`.
- **A raw `<input>` does NOT register as the control.** Replace bare inputs with `Field.Input` (v-model works: it declares
  `modelValue` + `update:modelValue`) and `textarea` with `Field.Textarea`. Otherwise the label points at a non-existent id.
- **Keep as-is** (deliberately not Ark): the mail search box (`SearchBar.vue`, hand-written on purpose), `Editable.Input`
  text fields, a hidden `<input type="file">` wrapped in its own `<label>` (implicit association), the native
  `<input type="color">` swatch row, and the tiny unlabelled filter box inside `LLMModelPicker`.
- **Layout classes stay local, styling is shared.** Row layouts (`.row`, `.frow`, `.d-field`) live in the component;
  the label/label-column/hint look comes from `styles/ui.css` (`.row__label`/`.flabel`, `.field`, `.field__label`,
  `.field__hint`).
- `Field.HelperText` (and `ErrorText`) render **`<span>`s** — give them `display: block` if they must stack.
- **A hint that belongs under the control needs its own row.** `.frow` is a flex row, so a `Field.HelperText` placed
  inside it would sit *beside* the control, next to the switch. Use `.frow--stacked` (grid `108px 1fr`) and give the
  hint `grid-column: 2` — see the auto-sync row in `AccountForm.vue`. That keeps the helper text inside `Field.Root`
  (where Ark can describe the control with it) without a wrapper element or a hand-tuned `padding-left`.
  `views/settings/Common.vue` has the same thing for the settings pages, where the label column is the global
  `.row__label` basis: `.row--stacked` = grid `240px 1fr`.
- **Three levels of "hint", pick by what the sentence is about.** (1) A standing fact about *one field* →
  `Field.HelperText` under that field (normal colour for context, `--color-warning` for the ones that block you);
  it also gives Ark an `aria-describedby` target (`field::v-N::helper-text`), so screen readers announce it on focus.
  (2) A standing fact about the *pane/account* that no single field owns → a muted `.pane-note` line. (3) Something
  that *just happened* (waiting for the browser, test failed, authorization succeeded) → a `.status` line: icon +
  text, coloured `--color-text-3` / `--color-success` / `--color-danger`, with `role="status"` (`role="alert"` for
  errors) so the change is announced. No borders, no background: a boxed banner around one sentence shouted louder
  than the form and shoved it around. Keep at most one status line per slot — the account form used to stack three
  independent banners that could all be true at once.
- **`.field__hint--warn` must be redeclared in a component that also styles its own hint class.** Scoped styles carry
  an extra `[data-v-…]` attribute selector, so a component-local `.frow-hint { color: … }` (specificity 0,2,0) beats
  the global `.field__hint--warn` (0,1,0) from `ui.css`. Declare `.field__hint--warn` *after* the local hint rule in
  that component's `<style scoped>`.
- Validation is still manual (no `createForm`); `Field.Root` accepts `invalid` / `required` / `disabled` when we need it.
- **One control per `Field.Root` — a list of controls gets its own container.** Every Ark form control inside a field
  inherits `ids.control` from the field context, so two of them end up with the *same* `id` (and the field's
  `label[for]` resolves to whichever comes first in the tree). This is what silently broke the model picker: its 17
  checkboxes and the “add model” text input all carried one id, so clicking a row activated the text input and no
  checkbox ever toggled. Rule of thumb: a `Field.Root` wraps exactly one control (plus its label/helper text); panels,
  lists and repeated rows are rendered as siblings of the field, not inside it. Check with
  `document.querySelectorAll('[id="…"]').length` while debugging — it must be 1.
- The same collision is harmless-but-invalid in the auto-sync row (`AccountForm.vue`: switch + number input in one
  `Field.Root`). The switch is first in tree order, so `label[for]` still hits it and the row behaves correctly; don't
  copy that shape into new code.

## Feedback / confirmation

- ✓ Transient messages: one module-level `toaster = createToaster({ placement: 'top-end', … })` in
  `stores/feedback.ts`; components keep calling `feedback.success/error/info/warn` and a single
  `<ToastHost/>` (Ark `Toaster` + `Toast.Root`) renders them anywhere in the app. There is no per-component toast state.
- Persistent inline request/validation context uses `components/ui/UiAlert.vue`; do not hand-roll
  another bordered red/warning box. Settings pages keep the shared `wrap → surface → surface__head`
  hierarchy, including Contacts.
- The toast root needs Ark's documented runtime styling (`width` — its group is a zero-width fixed flex column, so a
  content-sized root collapses CJK text one glyph per line — plus `translate: var(--x) var(--y)`, `scale`, `z-index`,
  `height`, `opacity`). It lives in `styles/ui.css` under `[data-scope='toast'][data-part='root']`.
- Confirmation: `ConfirmDialog` (or a dialog/popover when the interaction is custom). Dialogs share the `.dlg-*` shell in
  `styles/ui.css` instead of re-declaring backdrop/positioner/card CSS per component, and each one opts into Ark's
  render strategy (`lazy-mount` + `unmount-on-exit`) so its content only exists while open.
- **`UiDialog` cards are flush by default.** Keep `dlg-head` / `dlg-body` / `dlg-foot` as direct children of
  `UiDialog`; their sections own all padding, so no extra prop is required. Only compact dialogs made from raw
  title/description/actions content pass `padded` to restore the 16px card padding. An extra `<form>` wrapper breaks
  the card's flex layout and makes the dialog look unlike the other form dialogs. Handle Enter on the field and the
  confirm button directly instead of relying on a wrapper form.
- **Closed overlays are hidden with the `hidden` attribute, not unmounted** (`Dialog.Content`/`Backdrop`, popover/menu
  content). The UA `[hidden] { display: none }` loses to *any* author rule that sets `display`, and our
  `.dlg-card { display: flex }` did exactly that — a closed-but-mounted dialog stayed painted on top of the page.
  `global.css` therefore restates `[hidden] { display: none !important }`; keep that rule, and don't rely on a `v-if`
  per dialog.
- **`.dlg-body` is the scroll area, and it has to stay that way.** `.dlg-card` caps itself (`max-height: 100%`) and
  clips (`overflow: hidden`), so if the body keeps its natural height the card cuts the overflow off and the footer —
  the last child — is the first thing to vanish. Hence `.dlg-body { flex: 1 1 auto; min-height: 0; overflow-y: auto }`:
  `min-height: 0` is what lets a flex item shrink below its content at all. Symptom to remember: “the save button is
  gone once <some panel> opens”. Don't put `overflow: hidden` or a fixed height on a dialog body.

## Icons

The whole icon surface funnels through **`<AppIcon>`**
(`frontend/src/components/icon/AppIcon.vue`), not raw icon tags.
- Iconify icons are **offline UnoCSS classes** (`i-ri:sun-line`) from `@iconify-json/*` + `presetIcons`; `<AppIcon>` renders
  an inner `<i :class="`i-${icon}`">` glyph. We do NOT use `@iconify/vue` runtime `<Icon>` (no CDN fetch).
- Installed offline sets (as devDeps): `ri`, `eos-icons`, `streamline`, `streamline-color`, `logos`, `vscode-icons`
  (add `@iconify-json/<set>` if a needed prefix is missing).
- `lobehub` provider avatars / `ben` local images render as `<img>` (unchanged).
- **Dynamic icons** (runtime-chosen: attachment by mime, provider fallbacks) are driven by static maps but only referenced as
  strings at runtime — so they must also be published. Any icon name not a literal `i-*` token in source is listed in the
  UnoCSS **`safelist`** (`uno.config.ts`), which forces emission. When you add a runtime/map icon, add its class there too.
- Icon helper VNodes (`renderProviderIcon`, `ICON_MAPPING[*].create`) return `h('i', {class:'i-…'})` or `<img>`; keep them
  as the single source for runtime-chosen icons.

## Mail sidebar = TreeView (accounts → folders)

The account/folder sidebar is an expandable **tree**, not a menu:
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
  example in the Ark UI TreeView docs).
- `collapsed` param (app rail collapsed) is a separate CSS concern in `App.vue`/`TitleBar`, not prop plumbing into
  TreeView; reflow row widths via CSS once collapsed.

## Current conventions and verification

- Every dialog uses Ark `Dialog` + the shared `.dlg-*` shell.
- Every toast goes through the one `toaster` in `stores/feedback.ts`.
- Every labelled control sits in an Ark `Field.Root`; one `Field.Root` wraps one control.
- Shared controls live in `components/ui/`: `UiSwitch`, `UiCheckbox`, `UiNumberInput`,
  `UiPasswordInput`, `UiSelect`, `UiScrollArea`, `UiAlert`, `UiDialog` and `UiColorPicker`.
- Use global design tokens in `src/styles/global.css`; icons are offline `i-*`
  classes from `uno.config.ts` and the installed `@iconify-json/*` sets.
- After changing components, run `cd frontend && pnpm typecheck && pnpm lint && pnpm test`.
  `vite build` does not type-check `.vue` files.
