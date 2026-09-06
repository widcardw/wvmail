---
name: project-architecture
description: Overall project architecture for the wvmail Wails3 + Vue3 desktop email client application
type: project
---

# wvmail — Project Architecture

## Overview

wvmail is a cross-platform desktop email client built with **Wails3 (Go)** + **Vue 3 (TypeScript)**. The Go backend provides system-level services, while the Vue frontend renders the UI inside a native WebView.

- **Go Module**: `wvmail`
- **Wails Version**: `v3.0.0-alpha.84`
- **Build System**: Taskfile.yml (https://taskfile.dev)
- **Platform Targets**: macOS (darwin), Windows, Linux, Android, iOS

---

## Backend (Go)

### Entry Point

- `main.go` — application setup, window creation, event emission
- Registers a `GreetService` and emits a `time` event every second

### Services

- `services/greetservice.go` — `GreetService` with single `Greet(name string) string` method
- `services/configservice.go` — `ConfigService` for reading/writing YAML config with AES-256-GCM encrypted API keys
- `services/osservice.go` — `OsService` for OS-level operations (open files, folders, detect OS)
- All Go services live under `services/` as package `services`
- Services are registered via `application.NewService(&services.XxxService{})` in `main.go` and auto-exposed to the frontend

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

Location: `services/configservice.go`

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
│   ├── main.ts              # App entry: createApp + Pinia + mount
│   ├── App.vue              # Root: Naive UI NConfigProvider wrapper
│   ├── vite-env.d.ts        # Vite + .vue module type declarations
│   ├── components/
│   │   └── HelloWorld.vue   # Sample component (Wails bridge demo)
│   ├── router/
│   │   └── index.ts         # Vue Router setup (currently empty)
│   └── stores/
│       └── theme.ts         # Pinia store: Naive UI dark/light theme toggle
├── package.json
├── vite.config.js           # Vite config: Vue plugin, Wails plugin, @ alias
├── tsconfig.json            # TypeScript config (ESNext, bundler resolution)
├── .oxlintrc.json           # Linter rules
└── .oxfmtrc.json            # Formatter config (no semicolons, single quotes)
```

### Wails3 ↔ Frontend Bridge

1. **Service Calls** (Go → JS): Go services are imported as ES modules from auto-generated `bindings/`:
   ```ts
   import { GreetService } from '../../bindings/wvmail'
   GreetService.Greet(name).then(result => ...)
   ```

2. **Event System** (Go → JS): Go emits events via `app.Event.Emit(name, data)`. Frontend subscribes:
   ```ts
   import { Events } from '@wailsio/runtime'
   Events.On('time', (timeValue) => { ... })
   ```

3. **Vite Plugin**: `@wailsio/runtime/plugins/vite` processes the bindings directory and sets up dev-mode proxying.

### Theming & CSS (no library theme)

Ark UI provides no forest-level theme. `stores/theme.ts` resolves the effective mode (System syncs with OS dark via
`window.matchMedia`) and sets `data-theme="light|dark"` on `<html>`. Design tokens in `src/styles/global.css` flip
under both modes; runtime primary-color writes `--color-primary*`. See `.ai-docs/ark-ui-guide.md` / `arkui-vue.txt`.

### Icons

All icons go through `components/AppIcon.vue` (wraps `@iconify/vue` `<Icon>`, plus lobehub avatars & local `<img>`
sources). No naive `NIcon` involvement.

### Coding Conventions

- **Linter**: oxlint (rust-based), correctness category as warning, unused-vars as error
- **Formatter**: oxfmt, no semicolons, single quotes, ignores `bindings/`, `dist/`, `node_modules/`
- **TypeScript**: strict mode, ESNext target, bundler module resolution
