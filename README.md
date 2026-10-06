# WVMail

A cross-platform desktop email client built with **Wails3 (Go)** + **Vue 3 (TypeScript)** + **Ark UI** (headless) + **UnoCSS**.

![Tech Stack](https://img.shields.io/badge/Go-1.25-blue?logo=go)
![Wails](https://img.shields.io/badge/Wails-3.0.0--beta.25-purple)
![Vue](https://img.shields.io/badge/Vue-3.5-4FC08D?logo=vue.js)
![TypeScript](https://img.shields.io/badge/TypeScript-strict-3178C6?logo=typescript)

---

## Features

- **📧 Email via IMAP** — Manage multiple IMAP email accounts with local SQLite caching for folders and email summaries
- **📝 Notes** — Built-in note-taking with rich text editing (TipTap)
- **⏰ Reminders** — Quick reminder management
- **⚙️ Full Settings** — Theme, language, accounts, LLM provider configuration
- **🤖 LLM Integration** — Configure multiple LLM providers for AI-assisted features
- **🌐 i18n** — Typed zh-CN / en-US dictionary (`src/i18n`)
- **🎨 Theming** — light/dark/system + custom primary colour, driven by CSS custom properties

## Architecture

```
wvmail/
├── main.go                 # Entry point: window, tray, close behaviour, the one quit path
├── services/               # Go backend services (one package, one file per concern)
│   ├── configservice.go    # Config API (models, typed savers, accounts) + configstore.go + configllm.go
│   ├── emailservice.go     # IMAP: folders, bodies, attachments, search, decode, dial
│   ├── emailsyncservice.go # Background sync + tray unread badge
│   ├── networkservice.go   # Connection quality probe (title-bar meter)
│   ├── osservice.go        # Open files/folders/URLs, save dialog, QuitApp
│   ├── database*.go        # SQLite cache (GORM): mail, bodies, notes, reminders
│   └── notes/reminder/translation/logger services
├── frontend/               # Vue 3 + TypeScript frontend
│   └── src/
│       ├── App.vue         # Root layout (title bar + sidebar + routed view)
│       ├── components/     # ui/ (design system) + one directory per area
│       ├── composables/    # html/ (mail body pipeline), icons/, small pure helpers
│       ├── router/         # Vue Router (mail, notes, reminder, settings)
│       ├── stores/         # Pinia (accounts, theme, locale, preferences, connection, …)
│       ├── views/          # Page components
│       ├── types/          # TypeScript mirrors of the bound Go models
│       └── i18n/           # Locale definitions
├── build/                  # OS-specific build configs (macOS, Windows, Linux)
├── Taskfile.yml            # Build task definitions (Taskfile.dev)
└── go.mod                  # Go module
```

### Go Backend Services

| Service | Description |
|---|---|
| `ConfigService` | Read/write YAML config with encrypted API keys (AES-256-GCM) |
| `EmailService` | IMAP email operations (connect, list folders, fetch messages) |
| `NotesService` | Persist and retrieve notes |
| `OsService` | Open files/folders/URLs, save dialog, quit the app |
| `NetworkService` | Reachability + latency probe for the title-bar connection meter |
| `EmailSyncService` | Background sync loop + tray unread badge |
| `NotesService` / `RemindersService` / `TranslationService` | Notes, reminders, LLM translation |

### Frontend Pages

| Route | Page | Description |
|---|---|---|
| `/` | MailView | Main email client interface |
| `/notes` | NotesView | Rich text notes |
| `/reminder` | ReminderView | Quick reminders |
| `/settings/common` | SettingsCommon | Theme, language preferences |
| `/settings/accounts` | SettingsAccount | IMAP account management |
| `/settings/llm` | SettingsLLM | LLM provider configuration |
| `/settings/about` | SettingsAbout | App info |

## Prerequisites

- **Go** 1.25+
- **Node.js** & **pnpm**
- **Wails3 CLI** (`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`)
- Platform prerequisites (see [Wails docs](https://v3.wails.io/)):
  - **macOS**: Xcode Command Line Tools
  - **Windows**: WebView2 runtime (built into Windows 10+)
  - **Linux**: GTK3, WebKit2GTK

## Development

```bash
# Start dev mode with hot-reload (Vite on port 9245)
wails3 dev

# Regenerate JS/TS bindings after adding Go service methods
wails3 generate bindings -clean=true
```

## Build & Run

```bash
# Production build for current OS
wails3 task build

# Run the production build
wails3 task run

# Package into a distributable format
wails3 task package
```

### Server Mode

The app can also run as a headless HTTP server:

```bash
wails3 task build:server
wails3 task run:server
```

### Docker (Server Mode)

```bash
wails3 task build:docker
wails3 task run:docker
```

## Frontend Tooling

```bash
cd frontend

# Lint
pnpm lint           # Check
pnpm lint:fix       # Auto-fix

# Format
pnpm fmt            # Format with oxfmt (no semicolons, single quotes)

# Build only the frontend
pnpm build:dev      # Development build
pnpm build          # Production build
```

## Tech Stack

| Layer | Technology |
|---|---|
| **Desktop Framework** | [Wails3](https://v3.wails.io/) (v3.0.0-beta.25) |
| **Go Libraries** | go-imap, go-message, GORM + SQLite, go-yaml |
| **UI Framework** | Vue 3 (Composition API, `<script setup>`) |
| **Component Library** | [Ark UI](https://ark-ui.com/) (headless) + our own wrappers in `components/ui/` |
| **Styling** | UnoCSS + CSS Modules + design tokens (`styles/global.css`) |
| **State Management** | [Pinia](https://pinia.vuejs.org/) |
| **Router** | [Vue Router](https://router.vuejs.org/) |
| **Utilities** | [@vueuse/core](https://vueuse.org/) |
| **Rich Text** | [TipTap](https://tiptap.dev/) (Notes) |
| **Icons** | offline UnoCSS preset icons (`@iconify-json/*`), LobeHub avatars |
| **Linter** | oxlint |
| **Formatter** | oxfmt |
| **Package Manager** | pnpm |

## Config

Configuration is stored in the OS-specific config directory as YAML:

- **macOS**: `~/Library/Application Support/wvmail/config.yaml`
- **Linux**: `~/.config/wvmail/config.yaml`
- **Windows**: `%AppData%/wvmail/config.yaml`

API keys are encrypted at rest using AES-256-GCM.

## License

GNU Affero General Public License v3.0 (AGPL-3.0) — see [LICENSE](./LICENSE) for the full text.
