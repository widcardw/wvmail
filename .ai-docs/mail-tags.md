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
| `GetEmailTagState(accountID, folder, uid)` | Effective tags plus automatic matches for one message |
| `SetEmailTags(accountID, folder, uid, selectedTagIDs)` | Store only manual differences from the rules |
| `PreviewTagSuggestionPrompt(request)` | Build the exact AI prompt without a network request |
| `GenerateTagSuggestions(request)` | Send the generated prompt or the user-edited `prompt_override`, then validate the JSON rules |

### Rule semantics

- A rule has `keywords`, `fields`, a color and an enabled flag.
- Fields are `subject`, `from`, `domain`, and `body`.
- Keywords inside one tag are ORed. A message may match any number of tags.
- Classification-page filters are also ORed.
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
- `TagFilter.vue` is the multi-tag OR selector.
- `EmailListItem.vue` renders tag chips from `EmailSummary.tag_ids`.
- The message toolbar has a tag button; `EmailTagDialog.vue` edits manual tags.
- Settings → Tags (`views/settings/Tags.vue`) owns rule CRUD.

## AI suggestions and privacy

The AI path is opt-in only:

1. The user chooses accounts, sample limit, LLM/model, and optional fields.
2. Subject is always included; sender name, address, domain, date and a bounded
   body excerpt are off by default.
3. `PreviewTagSuggestionPrompt` returns the exact prompt and sample/account
   metadata. Opening or previewing never sends anything.
4. The dialog shows a prominent leakage warning and the full prompt in an editable,
   line-preserving textarea.
5. Only the explicit send button calls `GenerateTagSuggestions`; it sends
   `prompt_override` verbatim when present.
6. The backend strips Markdown fences, validates rules, and returns suggestions.
7. The user reviews selected suggestions before `SaveMailTag` writes them.

API keys are never inserted into prompts. Prompts, addresses and AI responses are
not logged.

## Shared LLM transport

`services/llmchat.go` owns the provider-specific chat-completion formats for
OpenAI-compatible APIs, Anthropic, Gemini and Ollama. Translation and tag
suggestions both use `requestLLMChat`; new LLM features should reuse it rather
than copying request code.

## Tests

- `services/tagservice_test.go`: OR matching, field scope, Gmail deduplication,
  explicit exclusion, response validation, preview/sent prompt equality, and
  verbatim prompt overrides.
- Run `GOCACHE=/tmp/wvmail-gocache go test ./...`.
