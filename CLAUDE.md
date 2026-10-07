# wvmail Project Guide

## Project Overview

Cross-platform desktop email client built with **Wails3 (Go)** + **Vue 3 (TypeScript)** + **Ark UI (headless)**.

## Key Conventions

- **Go services** go in `services/` with `package services` — one file per concern, not one file per service: `emailservice.go` (list path) / `emailfolders.go` / `emailbody.go` / `emailattachments.go` / `emailsearch.go` / `emaildecode.go` / `imapdial.go`, and `configservice.go` / `configstore.go` / `configllm.go`. Tests sit next to what they cover (`*_test.go`).
- **No sub-packages under `services/` (deliberate).** In Go a directory *is* a package, so filing files into subfolders means new import paths: everything shared must be exported, and the model types are the hub (`EmailAccount`/`EmailDetail`/`AttachmentInfo`/`Cached*` are referenced ~200× across every file), so `mail`↔`store` would import each other's types — an import cycle — unless a `internal/models` package is extracted first. Tests also have to stay in their package's directory (they use unexported helpers). Create a package only when all three hold: a nameable single responsibility, a small exported surface, and no reverse dependency (plus a second consumer — the server build is the same `main` with `-tags server`, so `internal/` buys nothing today). Splitting `services/` would also rename the Wails binding path, i.e. every `#/wvmail/services` import in the frontend.
- **Vue components** use Composition API (`<script setup lang="ts">`) and live in `src/components/<area>/` (one directory per area: `detail/ header/ icon/ list/ reminder/ settings/ sidebar/`); the reusable design-system primitives are `src/components/ui/` (`Ui*.vue`, `BusyButton.vue`), and only app-level singletons (`ToastHost.vue`) sit at the root.
- **Composables** in `src/composables/`: the mail-body HTML pipeline is grouped in `html/` (import the `html/index.ts` barrel, not its files), icon helpers in `icons/`, date/time detection in `date-mentions/` (entry `index.ts`); small pure helpers stay flat at the root (`search-query.ts` is pure and tested, `use-mail-search.ts` is its stateful half). Bare-Node tests must be able to load the chain, so relative imports inside those folders carry an explicit `.ts` and type-only imports use `import type` / inline `type`. Single-feature state machines live in `composables/` too (`mail-translation.ts`).
- **State management** uses Pinia stores in `src/stores/`, one store per concern: accounts/folders (`accounts.ts`), the message list and reading pane (`accounts.ts` for now), cached search (`search.ts`), plus `theme/locale/preferences/notes/reminders/feedback/connection`.
- **Routes** are defined in `src/router/index.ts`
- **Ark UI** (headless, per-component compound components like `<Slider.Root>`) is used for behavior; **all visual styling is explicit** — utility classes via **UnoCSS**, per-component CSS Modules / SFC `scoped`, plus global token variables in `src/styles/global.css`. There is **no baked-in theme**, so components are unstyled until we add CSS (see `.ai-docs/ark-ui-guide.md`).
- Icons are **offline UnoCSS preset-icons** from `@iconify-json/*` collections (e.g. `class="i-ri:sun-line"`), bundled at build time; rendered through the `<AppIcon>` wrapper (never raw icon markup). No `@iconify/vue` runtime/CDN fetches.
- **No semicolons**, **single quotes** (enforced by oxfmt)
- **TypeScript strict mode** enabled

## Development Commands

| Command | Purpose |
|---|---|
| `wails3 dev` | Dev mode with hot-reload (both Go + Vue via Vite) |
| `wails3 task build` | Production build for current OS |
| `go vet ./...` | Check Go code compiles |
| `cd frontend && pnpm typecheck` | Type-check `.vue` + `.ts` with vue-tsc. **Run it after touching components**: `oxlint` and `vite build` do not type-check |
| `cd frontend && pnpm test` | Node regression tests (`tests/*.test.mts`: sanitizer, date detection, badges, connection meter) |
| `cd frontend && pnpm lint` | oxlint |

## Documentation References

Additional docs are in `.ai-docs/`:
- `project-architecture.md` — Full project structure and tech stack (Ark UI + UnoCSS/headless)
- `vue-patterns.md` — Vue 3 Composition API patterns used in this project
- `ark-ui-guide.md` — Ark UI (headless) theming, import paths, CSS Modules + global.css tokens, and the naive→ark migration map
- `email-fetch-and-display.md` — IMAP fetch path, fallbacks, caching/pagination, decoding rules
- `mail-date-reminders.md` — date/time detection in mail bodies (chrono-node + guards), the highlight/tooltip markup, the click → reminder flow, and where reminders are stored
- `dev-debugging.md` — running the app, logging, macOS devtools/WebKit pitfalls
- `build-assets-and-icons.md` — app icon pipeline, `Assets.car` vs `icons.icns` precedence, actool/Xcode-license trap, `build/` vs CLI templates
- `window-tray-and-close.md` — window-close interception (`minimize_when_close`), tray menu semantics, and why it needs a Wails hook
- `config-store.md` — what `config.yaml` holds, secret encryption, atomic + serialized writes, and why it stays a single file
- `connection-indicator.md` — the title-bar connection meter: what the Go probe measures, its latency thresholds and polling schedule, and why it is not `dialIMAP`

## Design Tokens (global.css) + Theming

The design tokens that naive-ui used to inject at runtime (`--n-border-color`, `--n-text-color`, …) are now plain CSS custom properties defined in `src/styles/global.css` under `:root` (light) and `[data-theme='dark']`. `src/stores/theme.ts` toggles `data-theme` on `<html>`; there is no component-forest-level theme provider to thread through. When styling a part, reference tokens (`var(--border-color)`) — do not hardcode colors.

## Tool Usage Guidance

When working on this project:
- **Library documentation**: Use the public Ark UI docs (and the local reference dump in `.ai-docs/arkui-vue.txt`) for API specifics of `@ark-ui/vue`, and the docs for Vue, Vite, Pinia, Vue Router, @vueuse/core, UnoCSS. Do NOT rely on training data for API specifics.
- **Web search**: Use WebSearch or WebFetch when you need broader context (e.g., debugging a specific error, comparing approaches, checking compatibility).
