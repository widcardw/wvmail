# WVMail

A local-first, cross-platform desktop email client built with **Wails3 (Go)** +
**Vue 3 (TypeScript)** + **Ark UI** (headless) + **UnoCSS**.

![Tech Stack](https://img.shields.io/badge/Go-1.25-blue?logo=go)
![Wails](https://img.shields.io/badge/Wails-3.0.0--beta.25-purple)
![Vue](https://img.shields.io/badge/Vue-3.5-4FC08D?logo=vue.js)
![TypeScript](https://img.shields.io/badge/TypeScript-strict-3178C6?logo=typescript)

WVMail keeps mail, contacts, tags, notes and reminders in a local SQLite cache.
Remote content in mail is blocked by default; when the user allows it, images can
be fetched through an image proxy instead of the WebView.

## Features

- **Multi-account IMAP/SMTP** — folders, cache-first paging, read/flag/delete,
  attachments, search, Sent copies and per-account proxy overrides.
- **Real-time and fallback sync** — IMAP IDLE for auto-sync accounts, with
  interval polling as the fallback; tray unread badge and account-health markers.
- **Conversation view** — groups messages by `References`/`In-Reply-To`, with a
  bounded subject + participant fallback, one row per conversation and tabs for
  each message in the reading pane.
- **OAuth2** — online verification for Gmail and Microsoft accounts, alongside
  password-based IMAP/SMTP.
- **Privacy controls** — remote images/CSS blocked by default, per-sender
  allowlists, single-message reveal, suspicious-mail gate, CSS sanitization and
  an optional image proxy for hiding the reader's IP.
- **Tags** — five built-in categories (verification, billing, travel, schedule,
  subscriptions) plus local keyword rules, manual overrides and opt-in AI
  suggestions with a reviewable prompt.
- **Contacts** — local address book, sender-menu actions, exclusions after
  deletion and opt-in automatic collection from new INBOX mail.
- **Compose** — rich text, attachments, inline images, reply/reply-all/forward,
  `mailto:` handling and a single reusable compose window.
- **Notes and reminders** — TipTap notes, date/time detection in mail bodies and
  click-to-reminder.
- **AI assist** — translation and summarization through configured LLM providers
  with a privacy confirmation step.
- **i18n and theming** — typed `zh-CN` / `en-US` dictionaries, light/dark/system
  themes and a configurable primary colour.

## Architecture

```
wvmail/
├── main.go                  # App startup, window/tray wiring, single quit path, image-proxy middleware
├── services/                # Go backend, one package with one file per concern
│   ├── configservice.go     # Config API + configstore.go + configllm.go
│   ├── emailservice.go      # IMAP list/fetch/read/delete/flag/search
│   ├── emailthread*.go      # Conversation grouping and thread paging
│   ├── emailsend.go         # SMTP + MIME/CID
│   ├── composeservice.go    # Compose window, drafts and attachments
│   ├── contactservice.go    # Local contacts and INBOX auto-collection
│   ├── tagservice.go        # Built-in/user tags, classification and AI suggestions
│   ├── emailsyncservice.go  # Fallback polling + tray badge
│   ├── emailidle.go         # IMAP IDLE watchers
│   ├── imageproxy.go        # /image-proxy handler
│   ├── oauthservice.go      # Gmail/Microsoft OAuth flow
│   ├── database*.go         # SQLite cache (GORM)
│   └── notes/reminder/AI/OS/network/proxy/logger services
├── frontend/                # Vue 3 + TypeScript
│   └── src/
│       ├── App.vue          # Root layout (title bar + sidebar + routed view)
│       ├── components/      # ui/ design system + one directory per area
│       ├── composables/     # html/ sanitizer, icons/, mail-threads, formats, …
│       ├── router/          # Mail, compose, notes, reminders, settings
│       ├── stores/          # Pinia: accounts, contacts, tags, search, health, …
│       ├── views/           # Page-level components and settings pages
│       ├── types/           # TypeScript mirrors of bound Go models
│       └── i18n/            # Typed zh-CN / en-US dictionaries
├── build/                   # OS-specific Wails3 build configs
├── Taskfile.yml             # Build task definitions
└── go.mod                   # Go module
```

### Backend services

| Service | Responsibility |
|---|---|
| `ConfigService` | YAML config, encrypted secrets, account CRUD, LLM settings |
| `EmailService` | IMAP folders, messages, bodies, attachments, search, threads, read/flag/delete |
| `EmailSendService` | SMTP send, MIME/CID, best-effort Sent copy |
| `ComposeService` | Compose window lifecycle, in-memory drafts and attachment staging |
| `ContactsService` | Local address book and opt-in sender collection |
| `TagService` | Built-in/user tag rules, classification and AI suggestions |
| `EmailSyncService` / `EmailIdleService` | Fallback polling, IMAP IDLE and arrival events |
| `NetworkService` / `AccountHealthReporter` | Connection probe and per-account health signals |
| `OAuthService` | Gmail/Microsoft OAuth begin/poll/cancel |
| `NotesService` / `RemindersService` / `AIAssistService` | Notes, reminders and AI assist |
| `OsService` | Open files/folders/URLs, save dialogs and guarded quit |

The image proxy is not a Wails service; `main.go` mounts `services/imageproxy.go`
as middleware at `/image-proxy`.

### Frontend routes

| Route | Page | Description |
|---|---|---|
| `/` | `MailView` | Mailbox, conversation list, reading pane and compose entry |
| `/compose` | `ComposeView` | Separate compose window; the URL carries only the draft ID |
| `/notes` | `NotesView` | Rich-text notes |
| `/reminder` | `ReminderView` | Reminders |
| `/settings/common` | `SettingsCommon` | Theme, language, sidebar collapse and close behavior |
| `/settings/accounts` | `SettingsAccount` | Account setup, OAuth, sync and proxy |
| `/settings/contacts` | `SettingsContacts` | Local contacts and auto-collection |
| `/settings/security` | `SettingsSecurity` | Remote content, image proxy and safety controls |
| `/settings/proxy` | `SettingsProxy` | Global proxy and per-account routing |
| `/settings/llm` | `SettingsLLM` | LLM providers and models |
| `/settings/tags` | `SettingsTags` | Built-in/user tags and AI suggestions |
| `/settings/translation` | `SettingsTranslation` | Translation engine/model preferences |
| `/settings/about` | `SettingsAbout` | App info, version and log settings |

## Prerequisites

- **Go** 1.25+
- **Node.js** and **pnpm**
- **Wails3 CLI**:
  `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- Platform prerequisites (see [Wails docs](https://v3.wails.io/)):
  - **macOS**: Xcode Command Line Tools
  - **Windows**: WebView2 runtime (built into Windows 10+)
  - **Linux**: GTK3 and WebKit2GTK

## Development

```bash
# Start dev mode with hot-reload (Vite on port 9245)
wails3 dev

# Regenerate JS/TS bindings after changing Go service methods
wails3 generate bindings -clean=true
```

## Testing and checks

```bash
# Go services
go test ./services/...

# Frontend type-check, lint and regression tests
cd frontend
pnpm typecheck
pnpm lint
pnpm test
```

The frontend tests are plain Node tests (`tests/*.test.mts`); Node ≥ 23 strips
TypeScript types natively.

## Build and run

```bash
# Production build for the current OS
wails3 task build

# Run the production build
wails3 task run

# Package into a distributable format
wails3 task package
```

### Server mode

The same backend can run as a headless HTTP server:

```bash
wails3 task build:server
wails3 task run:server
```

### Docker (server mode)

```bash
wails3 task build:docker
wails3 task run:docker
```

## Frontend tooling

```bash
cd frontend

pnpm lint        # oxlint
pnpm lint:fix    # oxlint --fix
pnpm fmt         # oxfmt (no semicolons, single quotes)
pnpm build:dev   # Development frontend build
pnpm build       # Production frontend build
```

## Tech stack

| Layer | Technology |
|---|---|
| **Desktop framework** | [Wails3](https://v3.wails.io/) (v3.0.0-beta.25) |
| **Go libraries** | go-imap, go-message, GORM + SQLite, go-yaml |
| **UI framework** | Vue 3 (Composition API, `<script setup>`) |
| **Component library** | [Ark UI](https://ark-ui.com/) (headless) + wrappers in `components/ui/` |
| **Styling** | UnoCSS + CSS Modules + design tokens (`styles/global.css`) |
| **State management** | [Pinia](https://pinia.vuejs.org/) |
| **Router** | [Vue Router](https://router.vuejs.org/) |
| **Utilities** | [@vueuse/core](https://vueuse.org/) |
| **Rich text** | [TipTap](https://tiptap.dev/) |
| **Icons** | Offline UnoCSS preset icons (`@iconify-json/*`), LobeHub avatars |
| **Linter / formatter** | oxlint / oxfmt |
| **Package manager** | pnpm |

## Configuration

Configuration is stored as YAML in the OS-specific config directory:

- **macOS**: `~/Library/Application Support/wvmail/config.yaml`
- **Linux**: `~/.config/wvmail/config.yaml`
- **Windows**: `%AppData%/wvmail/config.yaml`

Account passwords, OAuth refresh tokens, LLM API keys and the DeepL auth key are
encrypted at rest with AES-256-GCM. The cache database, logs, avatars and staged
compose files live beside `config.yaml`.

## Documentation

Engineering notes live in [`.ai-docs/`](./.ai-docs/README.md). Start with:

- [Project architecture](./.ai-docs/project-architecture.md)
- [Mail fetch and display](./.ai-docs/email-fetch-and-display.md)
- [Compose and sending](./.ai-docs/compose-and-sending.md)
- [Config store](./.ai-docs/config-store.md)
- [Vue patterns](./.ai-docs/vue-patterns.md)
- [Ark UI guide](./.ai-docs/ark-ui-guide.md)

## License

GNU Affero General Public License v3.0 (AGPL-3.0) — see [LICENSE](./LICENSE) for
the full text.
