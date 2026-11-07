# wvmail engineering docs

These notes describe the code as it exists now. Source files and tests are the
source of truth; a note that disagrees with them is stale and should be fixed or
archived.

## Start here

| Doc | What it covers |
|---|---|
| `project-architecture.md` | Repository layout, services, stores, routes, Wails bridge and build commands |
| `email-fetch-and-display.md` | IMAP fetch/cache path, folder/status refreshes, body decoding/sanitization, oversized-content fitting, remote-content blocking and raw `.eml` export |
| `compose-and-sending.md` | Compose window, drafts, SMTP/MIME, attachments, reply/forward and `mailto:` |
| `config-store.md` | `config.yaml` fields, secret encryption, atomic writes and config events |
| `vue-patterns.md` | Vue/Pinia conventions, path aliases and Wails imports |
| `ark-ui-guide.md` | Ark UI primitives, shared wrappers, layout traps and styling rules |

## Feature notes

| Doc | What it covers |
|---|---|
| `ai-assist.md` | Translation/summarization privacy flow and the shared AI assist API |
| `mail-tags.md` | Built-in and user tags, local classification, Gmail deduplication and AI suggestions |
| `mail-date-reminders.md` | Date/time detection in mail bodies and click-to-reminder flow |
| `real-time-sync.md` | IMAP IDLE, heartbeat/watchdog, fallback polling and contact auto-collection wiring |
| `connection-indicator.md` | Title-bar connection meter and per-account health markers |
| `proxy.md` | Global proxy, per-account overrides and CONNECT/SOCKS5 dialing |
| `notes.md` | Notes editor lifecycle, autosave/flush rules and HTML-to-text conversion |
| `i18n.md` | Typed dictionaries, interpolation and plural rules |
| `window-tray-and-close.md` | Close-to-tray behavior, tray menu and the single quit path |
| `build-assets-and-icons.md` | macOS app icon pipeline and build-asset traps |
| `dev-debugging.md` | App logging and macOS/WebKit/Wails debugging notes |

## Historical and upstream material

- `archive/` contains dated reviews, research notes and implementation logs.
  They are kept for context, not as current implementation or API references.
- `reference/arkui-vue.txt` is a generated dump of upstream Ark UI documentation.
  It is not project documentation and may lag the installed package; prefer the
  official docs at <https://ark-ui.com/> when they disagree.

## Maintenance rules

- Update the note in the same change that alters the behavior it describes.
- Prefer stable anchors such as service names, method names and component names
  over line numbers.
- Delete superseded guides and one-off answers that have no ongoing value. Keep
  useful dated reviews or research in `archive/`; do not leave either category
  in the current index.
- When importing notes from `.workbuddy-ai` or similar scratch logs, extract only
  durable behavior, decisions and debugging conclusions into the relevant doc;
  do not copy dated session logs into `.ai-docs`.
- When moving or renaming a doc, update links in `README.md`, `CLAUDE.md` and the
  other notes that point to it. 
