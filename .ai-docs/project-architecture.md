---
name: project-architecture
description: Overall project architecture for the wvmail Wails3 + Vue3 desktop email client application
type: project
---

# wvmail — Project Architecture

## Overview

wvmail is a cross-platform desktop email client built with **Wails3 (Go)** + **Vue 3 (TypeScript)**. The Go backend provides system-level services, while the Vue frontend renders the UI inside a native WebView.

- **Go Module**: `wvmail`
- **Wails Version**: `v3.0.0-beta.25`
- **Build System**: Taskfile.yml (https://taskfile.dev)
- **Platform Targets**: macOS (darwin), Windows, Linux, Android, iOS

---

## Backend (Go)

### Entry Point

- `main.go` — application setup, window/tray wiring, the single quit path (`requestQuit`) and the
  close hook; see `.ai-docs/window-tray-and-close.md`

### Services

All Go services live under `services/` as package `services` and are registered via
`application.NewService(...)` in `main.go`, which auto-exposes their exported methods to the frontend.

| File(s) | Service | Concern |
|---|---|---|
| `services/configservice.go` / `configstore.go` / `configllm.go` | `ConfigService` | YAML config, AES-256-GCM secrets, LLM connection test + model list |
| `services/emailservice.go` / `emailfolders.go` / `emailbody.go` / `emailattachments.go` / `emailsearch.go` / `emaildecode.go` / `imapdial.go` | `EmailService` | IMAP fetch, decode, cache-first paging; see `.ai-docs/email-fetch-and-display.md` |
| `services/emailsend.go` / `emailmime.go` | `EmailSendService` | SMTP send, MIME/CID, best-effort Sent copy; see `.ai-docs/compose-and-sending.md` |
| `services/composeservice.go` / `composehtml.go` | `ComposeService` | single compose window, in-memory drafts, attachment staging, reply/forward/mailto entry |
| `services/database.go` / `databaseemail.go` / `databasebody.go` / `databasenotes.go` / `databasereminders.go` / `databasetags.go` | (cache) | SQLite (GORM) cache behind `EmailService` and `TagService` |
| `services/tagservice.go` / `llmchat.go` | `TagService` | local rule-based mail tags, OR classification, Gmail-view deduplication, privacy-reviewed AI suggestions; see `.ai-docs/mail-tags.md` |
| `services/emailsyncservice.go` | `EmailSyncService` | background sync per account + the tray unread badge |
| `services/networkservice.go` | `NetworkService` | reachability + latency probe behind the title-bar meter |
| `services/osservice.go` | `OsService` | open file/folder/URL, native save dialog, `QuitApp` |
| `services/notesservice.go`, `reminderservice.go`, `translationservice.go`, `logger.go` | | notes, reminders, LLM translation, file logging |

### Build Configuration

- `Taskfile.yml` — root task file dispatching to OS-specific Taskfiles under `build/`
- `build/config.yml` — Wails3 build config (app info, dev mode, file associations)
- OS-specific build logic in `build/darwin/`, `build/windows/`, `build/linux/`, `build/ios/`, `build/android/`

### Key Dev Commands

| Command | Description |
|---|---|
| `wails3 dev` | Development mode with hot-reload (uses Vite on port 9245) |
| `wails3 task build` | Production build for current OS |
| `wails3 task run` | Run the production build |
| `wails3 generate bindings -clean=true` | Regenerate JS/TS bindings after adding Go service methods |

---

## Go Service Development

### Adding a New Service Method

1. Add the method to the appropriate Go `*Service` type in `services/`
2. Run `wails3 generate bindings -clean=true` from the project root to regenerate frontend bindings
3. The new method is immediately available in the frontend via `import { XxxService } from '../../bindings/wvmail/services'`

### ConfigService

Location: `services/configservice.go` (typed API) + `configstore.go` (storage) + `configllm.go` (provider HTTP)

The config file is stored at `<os.UserConfigDir>/wvmail/config.yaml` as YAML.
API keys are encrypted with AES-256-GCM (master key in `<configDir>/.master.key`).

**Exported methods (available to frontend):**

| Method | Returns | Description |
|---|---|---|
| `GetConfig()` | `AppConfig` | Read config from disk (decrypts API keys) |
| `GetConfigDir()` | `string` | Path to the config directory |
| `SaveConfig(cfg)` | — | Write config to disk (encrypts API keys) |
| `SaveConfigField(field, value)` | — | Update a single field (`"language"` or `"theme"`) |
| `SaveLLMConfigs(configs)` | — | Replace the entire LLM config list |
| `TestLLMConnection(cfg)` | `[bool, string]` | Test LLM provider connectivity |

Field-level mutations go through `updateConfig(func(*AppConfig) error)` in
`configstore.go`: it owns the config lock, fresh read, mutation and atomic write.
Do not copy the lock/read/write sequence into a new service method. The exceptions
are `SaveConfig` (already has a complete config) and `SaveConfigField` (it also
reports whether the language changed for the tray event).

**Opening config folder in file manager (frontend):**

```ts
import { ConfigService, OsService } from '../../bindings/wvmail/services'

const dir = await ConfigService.GetConfigDir()
await OsService.OpenFolder(dir)
```

### OsService

Location: `services/osservice.go`

**Exported methods:**

| Method | Parameters | Description |
|---|---|---|
| `GetOs()` | — | Returns `"darwin"`, `"windows"`, or `"linux"` |
| `OpenFile(path)` | `string` | Open file with default OS application |
| `OpenFolder(path)` | `string` | Reveal folder in file manager (Finder, Explorer, etc.) |
| `PathStat(path)` | `string` | Returns `"file"`, `"dir"`, `"non-exist"`, or `"empty"` |

Config directory resolves to:
- **macOS**: `~/Library/Application Support/wvmail/`
- **Linux**: `~/.config/wvmail/`
- **Windows**: `%AppData%/wvmail/`

---

## Frontend (Vue 3)

### Tech Stack

| Layer | Technology | Version |
|---|---|---|
| UI Framework | Vue 3 | ^3.5.33 |
| Router | Vue Router | ^5.0.6 |
| State Mgmt | Pinia | ^3.0.4 |
| Behavior/Headless Lib | Ark UI | ^5.39.1 |
| Styling | UnoCSS + CSS Modules + global.css design tokens | frontend |
| Utilities | @vueuse/core | ^14.3.0 |
| Build Tool | Vite | ^8.0.10 |
| Vue Plugin | @vitejs/plugin-vue | ^6.0.6 |
| Linter | oxlint | ^1.62.0 |
| Formatter | oxfmt | ^0.47.0 |
| Runtime Bridge | @wailsio/runtime | latest |
| Package Manager | pnpm | (based on .pnpm store) |

### Directory Structure

```
frontend/
├── src/
│   ├── main.ts              # App entry: createApp + Pinia + global styles
│   ├── App.vue              # Root layout (title bar + sidebar + routed view)
│   ├── components/          # one directory per area
│   │   ├── ui/              # UiSelect / UiSwitch / UiBadge / … + BusyButton (design system)
│   │   ├── compose/         # Tiptap compose editor
│   │   ├── detail/ header/ icon/ list/ reminder/ settings/ sidebar/
│   │   └── ToastHost.vue    # app-level singleton (mounted once in App.vue)
│   ├── composables/
│   │   ├── html/            # mail-body HTML pipeline (index.ts is the barrel)
│   │   ├── icons/           # provider/brand/attachment icon helpers
│   │   └── *.ts             # small pure helpers (badge, connection, formats, date-mentions…)
│   ├── i18n/                # typed dictionary + message syntax (see .ai-docs/i18n.md)
│   ├── router/              # Vue Router: mail, compose window, notes, reminder, settings
│   ├── stores/              # Pinia: accounts, theme, locale, preferences, notes, reminders, connection
│   ├── styles/              # global.css (tokens) + ui.css (shared recipes: .btn, .icon-btn, .dlg-*)
│   ├── types/               # TS mirrors of the bound Go models
│   └── views/               # page-level components, including ComposeView (+ views/settings/)
├── tests/                   # node tests/*.test.mts (pnpm test)
├── index.html               # hashed entry; title + favicon
├── vite.config.js           # Vue plugin, Wails plugin, `~` → src, `#` → bindings
├── tsconfig.json            # strict TS
└── .oxlintrc.json / .oxfmtrc.json
```

### Wails3 ↔ Frontend Bridge

1. **Service Calls** (Go → JS): Go services are imported as ES modules from the generated
   `bindings/` directory, through the `#` alias (`#` → `bindings`):
   ```ts
   import { NetworkService } from '#/wvmail/services'

   const result = await NetworkService.ProbeConnection('imap.qq.com', 993, true)
   ```

2. **Event System** (Go → JS): Go emits events via `app.Event.Emit(name, data)`. Events with a typed
   payload are declared in `main.go`'s `init()`; the frontend subscribes through the runtime:
   ```ts
   import { Events } from '@wailsio/runtime'
   Events.On('emails-synced', (accountID: string) => { ... })
   ```

3. **Vite Plugin**: `@wailsio/runtime/plugins/vite` processes the bindings directory and sets up dev-mode proxying.

Outbound mail, the separate compose window, `mailto:` routing and MIME/CID handling are
documented in `.ai-docs/compose-and-sending.md`.

### Theming & CSS (no library theme)

Ark UI provides no forest-level theme. `stores/theme.ts` resolves the effective mode (System syncs with OS dark via
a reactive `window.matchMedia` result) and sets `data-theme="light|dark"` on `<html>`. The OS result must be stored in
a `ref`: reading `matchMedia().matches` directly inside a computed caches it and prevents live system-appearance changes.
Design tokens in `src/styles/global.css` flip under both modes; runtime primary-color writes `--color-primary*`.
See `.ai-docs/ark-ui-guide.md` / `arkui-vue.txt`.

### Icons

All icons go through `components/icon/AppIcon.vue`: offline UnoCSS preset icons (`.i-<name>`, bundled
at build time — never a runtime/CDN fetch), `@lobehub/icons-static-avatar` WebPs for LLM providers, and plain
`<img>` for local paths. Provider helpers live in `composables/icons/providers.ts`, brand/local mapping in
`composables/icons/mapping.ts`.

### Coding Conventions

- **Linter**: oxlint (rust-based), correctness category as warning, unused-vars as error
- **Formatter**: oxfmt, no semicolons, single quotes, ignores `bindings/`, `dist/`, `node_modules/`
- **TypeScript**: strict mode, ESNext target, bundler module resolution
