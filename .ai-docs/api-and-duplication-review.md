---
description: A 2026-11 review of the mail client's Go API surface and duplicated code, the refactors applied, and the items deliberately left for later
---

# API and duplication review (2026-11)

This note records a focused pass over the mail client path: the Wails services in
`services/` (IMAP folders/list/body/delete/flag/sync, tag filtering) and the Vue
components that drive them. Notes, reminders, OAuth and the build tooling were
only checked where they share helpers.

## Method

- Inventoried every exported method with `rg '^func .*\) [A-Z]' services/*.go` and
  compared it with the methods the frontend actually calls.
- Looked for repeated page bounds, repeated cache/refresh blocks, repeated MIME
  envelope formatting, and repeated message keys.
- Re-ran `wails3 generate bindings -clean=true` to see what the generator exposes
  (and to catch methods whose parameter types cannot cross the bridge).
- Verified with `go test ./services/...`, `pnpm typecheck`, `pnpm lint` and
  `pnpm test`.

## Findings fixed in this pass

### 1. Go-only wiring methods were part of the frontend API

Wails binds every exported method on a registered service. Three methods were
only meant to be called by `main.go` or by another Go service, yet each one
produced a JS binding:

| Method | Why it is not a frontend API | Fix |
|---|---|---|
| `EmailService.SetAccountHealthReporter(*accountHealthReporter)` | Takes an unexported type; only `main.go` wires it. | `//wails:ignore` |
| `EmailService.SetUnseenListener(func(...))` | Function parameter; only `EmailSyncService` wires it. | Renamed to unexported `setUnseenListener` |
| `ConfigService.SetAccountRemovedListener(func(...))` | Function parameter; only `main.go` wires it. The generator even warned "function types are not supported by encoding/json". | `//wails:ignore` |

`EmailIdleService.SetAccountHealthReporter` got the same directive for
consistency (that service is not currently registered, but the method has the
same shape).

After regenerating, the bindings contain 84 methods and no warnings.
`AccountHealthEvents` stays bound because the frontend calls it; the two setters
no longer appear.

### 2. Page bounds and slicing were copied into five places

`ListEmails`, `fetchEmailsOn`, `SearchEmails`, `ListTagFilteredEmails` and
`ListClassifiedEmails` each had their own `if page < 1 …` / `if pageSize < 1 …`
block. `EmailService.paginate` and `tagservice.paginateSummaries` were byte-for-
byte the same function.

New `services/emailpaging.go` owns:

- `normalizePage(page, pageSize)` — one defaulting rule.
- `paginateSlices[T](items, page, pageSize)` — one slicing rule, always returns a
  non-nil slice so JSON carries `[]` rather than `null`.
- `cacheCoversPage(...)` — moved here from `emailservice.go` so the cache rule
  has one home.

The two private paginators were deleted.

### 3. The cached-page rule had already drifted

`ListEmails` used `cacheCoversPage(page, pageSize, total, len(cached))`, while
`fetchEmailsOn` had an inline `page <= 1 || len(cached) >= page*pageSize ||
len(cached) >= total`. The two conditions agree today, but they were maintained
separately. `fetchEmailsOn` now uses `page <= 1 || cacheCoversPage(...)`, with the
page-1 exception explicit.

### 4. Folder cache/refresh logic was repeated

`ListFolders`, `FolderStatuses` and `DeleteEmail` each read `LoadFolders` and fell
back to `RefreshFolders` when the cache was empty. `DeleteEmail` additionally
needed to know whether the first load had already refreshed, so it could avoid a
second dial when looking for Trash.

`EmailService.loadFolderList` now owns the read-cache-then-refresh rule and
returns `(folders, refreshed, error)`. `ListFolders` delegates to it;
`FolderStatuses` and `DeleteEmail` use it directly. The sync loop and IDLE
watcher already called `ListFolders`, so they pick up the same rule without
changes.

### 5. Envelope formatting was duplicated and inconsistent

`emailSummaryFromBuffer` and `RefreshEmailBody` both formatted `From`/`To`/`Cc`
by hand. The copies had already diverged: the list path used the first `From`
header, while the detail path let the last one win.

New `services/emailenvelope.go` provides `envelopeAddress`,
`envelopeAddresses` and `envelopeSender`. Both paths now use the first `From`
(RFC 9051 allows one), and the list/detail views cannot disagree about a
provider's envelope shape. Tests cover host/bare addresses, order, display-name
formatting and the first-From rule.

### 6. The two tag-list endpoints repeated the classification pipeline

`ListTagFilteredEmails` and `ListClassifiedEmails` each loaded tags, filtered out
disabled rules, read emails/bodies/overrides, loaded folder attributes and called
`classifyCachedEmails`. The only real differences were the scope validation and
the response metadata.

Both now call `classifyTagPage(tagIDs, scopes, flaggedOnly)`, with
`enabledTagSelection` handling the enabled/selected intersection. An empty scope
list still means "the whole cache", which is what the aggregate Classified view
needs; a non-empty list still narrows the SQL reads.

### 7. The frontend message key was copied into three places

`stores/accounts.ts`, `EmailList.vue` and `ClassifiedEmailList.vue` each built
`account + folder + UID` keys with `\u200b` separators. New
`frontend/src/composables/mail-key.ts` exports `mailKey`, `mailKeyFor` and
`selectedMailKey`; the three call sites now share them. The fallback behaviour
(plain folder rows fall back to the selected account/folder, aggregate rows do
not) is preserved.

## Findings deliberately left for a later pass

These are real, but changing them either touches the Wails API in a way this
review did not justify, or is a larger model migration.

- **`ListFlaggedEmails(account, refresh bool)`** — the boolean hides a blocking
  IMAP search versus a SQLite-only read. A cleaner API would be
  `GetFlaggedSnapshot` plus `RefreshFlaggedSnapshot`, but that changes the
  binding and two frontend call sites.
- **`DBManager.DeleteEmail` vs `EmailService.DeleteEmail`** — same name, different
  layers (cache row vs server move). Internal-only; rename the DB method to
  `DeleteCachedEmail` the next time that file is touched.
- **Unused frontend surface** — `ConfigService.SaveAccounts`,
  `ConfigService.SaveConfig` and `ConfigService.MinimizeWhenClose` are exported
  and bound, but the frontend calls none of them (`SaveAccounts` only appears in a
  Go test). `MinimizeWhenClose` is Go-only wiring for the window close hook.
  Hiding them with `//wails:ignore` would shrink the bridge surface; they were
  left alone to keep this commit focused.
- **Folder keys** — `SideBar.vue` (`fkey`) and `FolderTreeNode.vue` build the same
  `account + \u200b + folder` key. A `folderKey` composable is the natural next
  dedup, but it touches the sidebar tree rather than the mail path.
- **`EmailSummary` / `EmailDetail` overlap** — both carry `From`/`FromName`/
  `FromAddr`/`To`. Merging them is a model migration, not a mechanical dedup.
- **`NewAccountHealthReporter`** returns an unexported type from an exported
  function. It works for `main.go`, but an exported interface or type would be a
  cleaner Go API.
- **`go build ./...`** still fails on `build/ios` ("function main is undeclared")
  because that directory is a platform stub. This is pre-existing and unrelated
  to the mail services; `go build ./services/... .` succeeds. The macOS linker
  version warnings are also pre-existing.

## Verification

| Check | Result |
|---|---|
| `go test ./services/...` | pass |
| `cd frontend && pnpm typecheck` | pass |
| `cd frontend && pnpm lint` | 0 warnings, 0 errors |
| `cd frontend && pnpm test` | pass |
| `wails3 generate bindings -clean=true` | 84 methods, no warnings |

## Suggested order for the follow-ups

1. Rename the DB-level `DeleteEmail` to `DeleteCachedEmail`.
2. Hide the unused `ConfigService` methods listed above.
3. Split `ListFlaggedEmails` into read/refresh methods if the API churn is
   acceptable.
4. Extract `folderKey` for the sidebar.
