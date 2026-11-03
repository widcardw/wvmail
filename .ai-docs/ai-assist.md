---
name: ai-assist
description: Translation and summarization privacy flow, provider selection, and the AI assist config/API
type: project
---

# AI Assist

The reading-pane toolbar has two AI actions:

- **Translate** (`ri:translate-2`) keeps the existing HTML-layout translation
  path and can use an LLM or DeepL.
- **Summarize** (`ri:sparkling-line`) sends the message text to an LLM and
  renders a Markdown summary (overview, key points, decisions, action items and
  deadlines). Summaries never use DeepL.

## Privacy confirmation

Before either action sends mail content off-device, `AIAssistConfirmDialog.vue`
shows:

- which provider will receive the message;
- the model when the provider is an LLM;
- a warning that the message may contain personal or confidential information;
- a **Don’t ask again** checkbox.

The dialog defaults to the provider/model saved in Settings → AI Assistance. If
DeepL is the configured translation engine and a key is present, DeepL is the
default translation choice; otherwise the configured LLM is selected.

When the user confirms with **Don’t ask again**, the preference is persisted as
the top-level config field `skip_ai_assist_confirmation`. The same field is
controlled by the Ark UI switch on Settings → AI Assistance. Once enabled,
clicking either toolbar action starts the request immediately.

## API and configuration

`services/aiassistservice.go` owns both operations. The frontend sends only an
`AIAssistRequest`:

```text
Text, TargetLang, Engine, LLMConfigName, LLMModel
```

Empty fields mean “use Settings → AI Assistance”. The Go service loads the
config, resolves the provider and key, and calls the shared `requestLLMChat`
path. This removes provider/key handling from the Vue layer and keeps the
dialog, direct execution and settings page on one source of truth.

Translation and summary results are cached for one hour. The cache key includes
the operation, text, target language, engine, provider, base URL and model, so
switching models cannot return another model's result. Errors are not cached.

The config type remains `TranslationConfig` and the YAML block remains
`translation_config`: renaming either would require a migration and would risk
silently dropping existing translation settings. The UI label is **AI
Assistance**; the route stays `/settings/translation` for the same
compatibility reason.

## Verification

`services/aiassistservice_test.go` covers summary model selection, successful
response caching, per-model cache separation, and the confirmation preference
round-trip. Frontend checks: `pnpm typecheck`, `pnpm lint`, `pnpm test`.
