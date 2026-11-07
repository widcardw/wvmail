# Config Store (`<os.UserConfigDir>/wvmail/config.yaml`)

One YAML file holds everything the app persists: UI preferences, behaviour
switches, the email accounts, the LLM providers and the translation settings.
This note records why it is still a single file, and the two properties that make
it safe to write.

## Layout

```
<os.UserConfigDir>/wvmail/             0700
├── config.yaml       0600  everything below
├── .master.key       0600  32 random bytes, AES-256-GCM key for the secrets
├── cache.db                 email cache (SQLite)
├── logs/
├── avatars/
└── compose/                 staged compose attachments (while a draft is open)
```

Typical paths:

- macOS: `~/Library/Application Support/wvmail/`
- Linux: `~/.config/wvmail/`
- Windows: `%AppData%/wvmail/`

## Fields

Top-level `AppConfig` fields:

- `language`, `theme`, `primary_color`
- `database_path`, `log_config`, `translation_config`
- `scan_external_safety`, `search_tag_delete_presses`
- `llm_configs`, `email_accounts`, `remote_content`
- `minimize_when_close`, `detect_dates_in_mail`, `skip_ai_assist_confirmation`
- `collapse_sidebar_smart_folders`, `collapse_sidebar_utilities`
- `auto_collect_contacts`
- `proxy`

`remote_content` controls mail-body resources:

- `allow_remote_content` — global opt-in to load remote images/CSS; the default
  is block.
- `allowed_senders` — lower-cased addresses or domain patterns that are always
  allowed.
- `image_proxy_url` — optional base URL for the image proxy. Empty uses the app's
  own `/image-proxy`; the desktop build can point this at a server-mode
  deployment to hide the reader's IP as well.

`email_accounts[]` keeps the sidebar label and the From-header identity separate:

- `name` is the local account label shown in the sidebar and must not leak into
  sent mail.
- `sender_name` is the display name recipients see in the `From` header.
- `email`, provider, IMAP/SMTP settings, `use_tls`, `sync_interval`, `avatar`.
- `auth_type` (`password` or `oauth2`), `oauth_provider`, `oauth_client_id`,
  `oauth_refresh_token` for Gmail/Microsoft online verification.

Two blocks are optional and omitted until the user opts in (`omitempty` on a zero
struct works in yaml.v3, which a test pins):

- `proxy: {enabled, url}` — the app-wide proxy; see `.ai-docs/proxy.md`.
- `email_accounts[].proxy: {mode, url}` — one account's override of it. An absent
  block means "follow the global proxy", which is what every account written
  before this option existed gets.

`SaveConfigField` supports these single-field updates: `language`, `theme`,
`primary_color`, `scan_external_safety`, `search_tag_delete_presses`,
`allow_remote_content`, `allowed_remote_senders` (JSON array),
`image_proxy_url`, `minimize_when_close`, `detect_dates_in_mail`,
`skip_ai_assist_confirmation`, `collapse_sidebar_smart_folders`,
`collapse_sidebar_utilities`, `auto_collect_contacts`, and `proxy` (JSON
`{"enabled":bool,"url":string}`). Account CRUD and list replacement go through
the typed account methods instead.

**Secrets are encrypted, not hidden.** `IMAPPass`/`SMTPPass`,
`OAuthRefreshToken`, `LLMConfigs[].APIKey` and
`TranslationConfig.DeepLAuthKey` are AES-256-GCM ciphertexts (base64 with the
nonce prefixed); the key lives in `.master.key`, not in the YAML. A decryption
failure leaves the raw value alone, so a config written before encryption existed
still works. Proxy credentials are the deliberate exception and remain plain text
inside the proxy URL.

## Writes are atomic

`writeFileAtomic` writes a temp file in the same directory, `Sync()`s it, then
renames it over `config.yaml`. This matters more than it looks: every settings
change rewrites the *whole* file after decrypting and re-encrypting every secret,
and the previous implementation used `os.WriteFile`, which truncates first — a
crash or a full disk in that window destroyed the accounts and keys, not just the
preference being changed. Rename is atomic within a filesystem, so a reader either
sees the old file or the new one, never a partial one.

## Writes are serialized

`configWriteMu` guards every mutating method. Each one is a read-modify-write
(`readConfig` → mutate → `writeConfig`), so without the lock two concurrent saves
(e.g. language and theme, or "add account" while the sync loop saves) can read the
same base config and the second write silently drops the first one's change.
Readers (`GetConfig`, `GetAccounts`) do not take the lock — the rename already
guarantees they see a complete file — and the internal helpers `readConfig` /
`writeConfig` are the only ways to touch disk from inside a locked section, which
is what keeps the lock from being taken twice (it is not reentrant).

`SaveConfigField` is the exception that proves the rule: it releases the lock
before emitting the `language-changed` event, so listeners cannot run inside the
critical section.

## Why one file (rather than splitting)

Splitting into `prefs.yaml` / `accounts.yaml` / `llm.yaml` was considered and
deliberately **not** done:

- The two real hazards were the non-atomic write and the unserialized
  read-modify-write, both fixed above. Splitting would only have shrunk the blast
  radius, at the cost of a migration that itself can lose data.
- Background writers are not a problem today: the sync service only *reads*
  accounts (`GetAccounts`); all writes come from UI actions.
- There is no external consumer that wants a subset (no sync/export feature yet).
  Sharing "preferences only" would be the first real reason to split — and secrets
  are already ciphertext, so the leak would be limited to account addresses.

If it is ever split, do it as a versioned migration: add `config_version`, copy
`config.yaml` to `config.yaml.bak` before writing the new files, and keep
`GetConfig` able to read the old single file so a downgrade does not lose data.

## Events

`SaveConfigField("language", …)` emits `language-changed` (registered in
`main.go`) after the file is written. The system tray builds its menu before the
webview exists and therefore cannot use the frontend's i18n, so it listens for
this event and rebuilds its labels at runtime instead of at the next launch.

Successful automatic contact collection emits `contacts-updated` with the number
of newly added contacts. `App.vue` listens for it so the Contacts page and sender
menus can refresh without polling.
