---
name: mail-tags
description: Local rule-based mail classification, Gmail deduplication, and privacy-reviewed AI tag suggestions
type: project
---

# Mail tags and classification

Tags are local labels, not IMAP folders. One message may match several tags, and
mail is never moved on the server.

## Backend

`services/tagservice.go` exposes `TagService`:

| Method | Purpose |
|---|---|
| `ListMailTags` / `SaveMailTag` / `DeleteMailTag` | CRUD for local rules |
| `ListClassifiedEmails(tagIDs, page, pageSize)` | OR-filter classified mail and return one page |
| `ListTagFilteredEmails(request)` | Filter one paginated mail-list scope by tags before counting or paging |
| `GetEmailTagState(accountID, folder, uid)` | Effective tags plus automatic matches for one message |
| `SetEmailTags(accountID, folder, uid, selectedTagIDs)` | Store only manual differences from the rules |
| `PreviewTagSuggestionPrompt(request)` | Build the exact AI prompt without a network request |
| `GenerateTagSuggestions(request)` | Send the generated prompt or the user-edited `prompt_override`, then validate the JSON rules |

### Built-in tags

`services/tagbuiltins.go` seeds five built-in rules on the first `ListMailTags`
call. They are normal local tags once inserted, so the user can edit their
keywords, fields, colour or enabled state; they cannot be deleted. The database
uses `CreateMailTagIfMissing`, so an existing user-edited row is never
overwritten and concurrent first loads are harmless.

| ID | Default label | Fields |
|---|---|---|
| `builtin:verification` | 验证消息 | subject, from, body |
| `builtin:billing` | 账单 | subject, from, body |
| `builtin:travel` | 航班与出行 | subject, body |
| `builtin:schedule` | 日程 | subject, body |
| `builtin:subscription` | 订阅与资讯 | subject, from, body |

The verification rule covers reset-password, verification-code, sign-in and
account-security mail. Billing covers invoices, receipts, payments and orders;
travel covers flights, hotels, tickets and itineraries; schedule covers calendar
invites, meetings and appointments; subscription covers newsletters, digests and
unsubscribe mail. These are seed rules, not hard-coded classifications: all
matching still goes through the same keyword/field pipeline as user tags.
The default labels are currently Chinese strings in `tagbuiltins.go`; the
“Built-in” badge itself is localized by the frontend.

### Rule semantics

- A rule has `keywords`, `fields`, a color and an enabled flag.
- Fields are `subject`, `from`, `domain`, and `body`.
- Keywords inside one tag are ORed. A message may match any number of tags.
- Classification-page filters are also ORed.
- Folder and aggregate-list filters use `current view AND (selected tags OR)`.
  The view is a folder, one matching folder per account for All Inbox/Sent/Drafts,
  or the complete cache with `flagged_only` for All Starred.
- `ListTagFilteredEmails` applies the view scope, enabled tags, manual overrides,
  body rules, and optional starred flag before pagination. Its returned `total`
  and page count therefore describe the filtered result, not the unfiltered page.
- Search keeps its own result set and does not expose the list tag-filter button.
- Comparisons are case-insensitive substring matches.
- `body` only reads already cached text. Classification never silently opens IMAP
  to download a body.
- Empty keyword/field lists do not match. Disabled tags are ignored.
- New mail is classified locally whenever the category view is read. There is no
  AI call in sync, list loading, or local classification.

### Manual overrides

`EmailTagOverride` stores only a difference from the automatic result:

- automatic false → selected true: an `Included=true` row
- automatic true → selected false: an `Included=false` row
- same result: no row

No row means "keep following the rule", so later rule edits continue to affect
that message.

### Gmail and duplicate views

Gmail caches the same message under `INBOX`, `Important`, and `All Mail`.
`classifyCachedEmails` groups by `account_id + MessageID` when a MessageID exists,
otherwise by `account_id + folder + uid`. The row kept for opening the message
prefers:

1. `INBOX`
2. another non-All folder
3. `\All`

This prevents duplicate category rows while retaining a folder/UID that can be
opened.

## Frontend

- Sidebar smart row: `分类` / `Categories` (`smartFolder === "classified"`).
- `ClassifiedEmailList.vue` uses the existing list/detail split and pagination.
- `TagFilter.vue` is the multi-tag OR selector. Its long tag list uses
  `UiScrollArea`, so the popover keeps one opaque surface and shows only the
  shared narrow scrollbar while hovered or scrolling.
- `EmailList.vue` places a compact `TagFilter` immediately after refresh. The
  selection is shared across folder and aggregate views, but is separate from the
  full-width filter on the Categories page. Search hides the button.
- `EmailListItem.vue` renders tag chips from `EmailSummary.tag_ids`.
- The message toolbar has a tag button; `EmailTagDialog.vue` edits manual tags.
- Settings → Tags (`views/settings/Tags.vue`) owns rule CRUD. Each row is kept
  compact and single-line: colour swatch, label, built-in/disabled badges,
  matched fields, keyword count, then edit/delete actions. The full keyword list
  is only shown in the row's `title` tooltip; it is not expanded in the page.
- Built-in tags show a localized “Built-in” badge and hide the delete action.

## AI suggestions and privacy

The AI path is opt-in only:

1. The user chooses accounts, sample limit, LLM/model, and optional fields.
2. Subject is always included; sender name, address, domain, date and a bounded
   body excerpt are off by default.
3. Prompt samples are deduplicated by message identity and by normalized subject.
   Subject comparison ignores case, repeated whitespace, and common reply/forward
   prefixes (`Re:`, `Fwd:`, `回复:`, `回复：`, `转发:`, `转发：`). Empty subjects
   remain separate, and the sample limit is applied after subject deduplication.
4. `PreviewTagSuggestionPrompt` returns the exact prompt and sample/account
   metadata. It does not require an LLM configuration: users can assemble and
   review the prompt before choosing where it will be sent.
5. The dialog uses the shared `UiScrollArea` for its full-height body, shows a
   prominent leakage warning, and keeps the full prompt in an editable,
   line-preserving textarea. LLM/model selectors appear below the prompt and do
   not invalidate it when changed.
6. Only the explicit send button calls `GenerateTagSuggestions`; it requires the
   selected LLM/model and sends
   `prompt_override` verbatim when present.
7. Tag suggestion requests reserve at least 8192 output tokens. Reasoning models
   can otherwise spend the entire completion budget before returning visible JSON.
   Empty or truncated responses produce an actionable error instead of the generic
   "no JSON array" message.
8. Development builds log the complete LLM prompt and raw provider response even
   when the configured log level would normally hide it. These diagnostics are
   compiled out under the `production` build tag and have no UI setting. They can
   contain private mail metadata and must only be used locally.
9. The backend strips Markdown fences, repairs common model formatting slips
   (trailing commas and a missing closing object brace before the array end),
   validates rules, and returns suggestions.
10. The user reviews selected suggestions before `SaveMailTag` writes them.

API keys are never inserted into prompts. Prompts, addresses and AI responses are
not logged in production; the development-only diagnostics described above are
compiled out of release builds.

## Shared LLM transport

`services/llmchat.go` owns the provider-specific chat-completion formats for
OpenAI-compatible APIs, Anthropic, Gemini and Ollama. Translation and tag
suggestions both use `requestLLMChat`; new LLM features should reuse it rather
than copying request code.

## Tests

- `services/tagservice_test.go`: OR matching, field scope, Gmail deduplication,
  explicit exclusion, response validation, preview/sent prompt equality, and
  verbatim prompt overrides, normalized-subject sample deduplication, and applying
  the sample limit after deduplication. It also verifies that previewing works
  without an LLM while generation still requires one, and that tag requests reserve
  enough output budget for reasoning models.
- Run `GOCACHE=/tmp/wvmail-gocache go test ./...`.
