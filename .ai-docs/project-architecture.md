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
- All Go services live under `services/` as package `services`
- Services are registered via `application.NewService(&services.GreetService{})` and auto-exposed to the frontend

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

---

## Frontend (Vue 3)

### Tech Stack

| Layer | Technology | Version |
|---|---|---|
| UI Framework | Vue 3 | ^3.5.33 |
| Router | Vue Router | ^5.0.6 |
| State Mgmt | Pinia | ^3.0.4 |
| Component Lib | Naive UI | ^2.44.1 |
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

### Naive UI Theme System

- `App.vue` wraps content in `<NConfigProvider :theme="theme.value">`
- `stores/theme.ts` detects OS theme via `useOsTheme()` (from Naive UI) and stores current theme
- A toggle button switches between `darkTheme` and `null` (light)

### Coding Conventions

- **Linter**: oxlint (rust-based), correctness category as warning, unused-vars as error
- **Formatter**: oxfmt, no semicolons, single quotes, ignores `bindings/`, `dist/`, `node_modules/`
- **TypeScript**: strict mode, ESNext target, bundler module resolution
