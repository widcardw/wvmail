# WVMail

A cross-platform desktop email client built with **Wails3 (Go)** + **Vue 3 (TypeScript)** + **Naive UI**.

![Tech Stack](https://img.shields.io/badge/Go-1.25-blue?logo=go)
![Wails](https://img.shields.io/badge/Wails-3.0.0--alpha.84-purple)
![Vue](https://img.shields.io/badge/Vue-3.5-4FC08D?logo=vue.js)
![TypeScript](https://img.shields.io/badge/TypeScript-strict-3178C6?logo=typescript)

---

## Features

- **📧 Email via IMAP** — Manage multiple IMAP email accounts with local SQLite caching for folders and email summaries
- **📝 Notes** — Built-in note-taking with rich text editing (TipTap)
- **⏰ Reminders** — Quick reminder management
- **⚙️ Full Settings** — Theme, language, accounts, LLM provider configuration
- **🤖 LLM Integration** — Configure multiple LLM providers for AI-assisted features
- **🌐 i18n** — Multi-language support via Naive UI's locale system
- **🎨 Naive UI** — Polished component library with dark/light theme toggle

## Architecture

```
wvmail/
├── main.go                 # Application entry point (Go + Wails3)
├── services/               # Go backend services
│   ├── configservice.go    # Config with AES-256-GCM encrypted API keys
│   ├── emailservice.go     # IMAP email handling
│   ├── notesservice.go     # Notes CRUD
│   ├── osservice.go        # OS operations (open files/folders, detect OS)
│   ├── greetservice.go     # Sample service
│   └── database.go         # SQLite database setup (GORM)
├── frontend/               # Vue 3 TypeScript frontend
│   └── src/
│       ├── App.vue         # Root: Naive UI layout with sidebar + title bar
│       ├── router/         # Vue Router (mail, notes, reminder, settings)
│       ├── stores/         # Pinia (theme, locale, accounts, notes)
│       ├── views/          # Page components
│       ├── components/     # Reusable components (SideBar, TitleBar, EmailList, etc.)
│       ├── types/          # TypeScript interfaces
│       ├── composables/    # Vue composables (icons)
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
| `OsService` | Open files/folders, detect OS, path stat |
| `GreetService` | Demo service |

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
| **Desktop Framework** | [Wails3](https://v3.wails.io/) (v3.0.0-alpha.84) |
| **Go Libraries** | go-imap, go-message, GORM + SQLite, go-yaml |
| **UI Framework** | Vue 3 (Composition API, `<script setup>`) |
| **Component Library** | [Naive UI](https://www.naiveui.com/) |
| **State Management** | [Pinia](https://pinia.vuejs.org/) |
| **Router** | [Vue Router](https://router.vuejs.org/) |
| **Utilities** | [@vueuse/core](https://vueuse.org/) |
| **Rich Text** | [TipTap](https://tiptap.dev/) (Notes) |
| **Icons** | Iconify, LobeHub Icons |
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
