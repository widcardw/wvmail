---
description: Detecting date/time expressions inside email bodies, highlighting them in the shadow body, and turning a click into a reminder (chrono-node parser, RemindersService, settings)
---

# Mail dates → reminders

A message that says `3月5日下午3点 评审` or `Board call Mar 5, 2026 at 3:30 PM` renders those
moments as clickable chips. Clicking one opens a confirmation dialog prefilled with the mail's
subject and the resolved time; confirming stores a reminder.

```
email body ──▶ sanitizeEmailHtml()                       ──▶ shadow body
                ├─ rehypeAutoLink      (before sanitize)      <span class="date-mention"
                ├─ rehypeSanitize                                data-remind-at="1772694000"
                ├─ rehypeSanitizeEmailCss                        data-remind-text="3月5日下午3点">
                ├─ rehypeBlockRemoteContent                  click
                └─ rehypeDateMentions  (after sanitize)  ──▶ EmailDetail.handleLinkClick
                                                              └─▶ RemindFromMailDialog
                                                                    └─▶ reminders store ──▶ SQLite
```

## Parsing: chrono-node, not a hand-rolled grammar

`frontend/src/composables/date-mentions.ts` answers "what moments are in this text?" by running
**both** the Chinese and the English `chrono-node` parsers (`chrono.zh.casual`, `chrono.en.casual`)
and keeping the strongest match where they overlap. Both always run — mail mixes languages
regardless of the UI language — and the app locale only breaks exact ties.

Using the library is what keeps this small. It already carries the locale tables (month names, day
periods, weekday words) and its own validation, so we get these for free:

| Input | Result |
|---|---|
| `2026-03-05 15:30`, `2026/03/05`, `2026.03.05` | full date + time |
| `3月5日下午3点`, `明天下午两点`, `下周三` | resolved Chinese |
| `Mar 5, 2026 at 3:30 PM`, `tomorrow 9am`, `9:05` | resolved English |
| `2月30日`, `2026年13月45日` | **rejected** (calendar validation) |
| `v1.2.3`, `192.168.1.1`, `$1,234.56`, `138 0013 8000`, `第3章`, `1.5 小时之后` | **rejected** |

The last row is the important one: those are the false positives that would otherwise force us to
write dozens of special cases.

On top of the parsers we filter, because a *highlight* is a promise that something is clickable:

- **Need a day or a clock time.** `isCertain('day')` / `isCertain('month')` / `isCertain('hour')`,
  otherwise a bare weekday (`周四`, `Thursday`) is a vague reference, not a moment.
- **Nothing glued to a bigger number.** A match may not start right after a digit, `.` or currency
  symbol, nor end right before a digit. (`1.5 小时之后` is why the `.` case exists — chrono reads it
  as `5 小时之后`.)
- **Plausibility window.** Past matches (older than a minute) are dropped; so are matches more than
  five years out. Relative mentions already roll forward via `forwardDate: true`.
- **One chip per moment.** Overlapping candidates collapse: most certain fields, then longest text,
  then the UI language's parser.

Deliberately **out of scope** (say the word before reaching for more regex): ranges
(`3月5-7日`), durations (`两周后`), recurrence, timezones, lunar dates. chrono-node handles a few of
those already, but we do not promise them.

`frontend/tests/date-mentions.test.mts` is table-driven with a fixed reference date
(2026-03-01 10:00, a Sunday): **a new format is one new row**, not a new branch.

## Highlighting

`rehypeDateMentions` in `frontend/src/composables/html-sanitize.ts` walks text nodes (skipping
`a`/`code`/`pre`/`style`/`script`, and never re-wrapping its own output) and replaces each match with:

```html
<span class="date-mention" data-remind-at="1772694000" data-remind-text="3月5日下午3点"
      title="点击加入提醒事项：…">3月5日下午3点</span>
```

Two properties of this pass matter:

- It runs **after** `rehypeSanitize`, like the CSS and remote-content passes. The schema therefore
  needs no `span`/`data-*` allowance, and an email cannot fake a chip: `class="date-mention"` and
  `data-remind-at` are stripped from the message's own markup (covered by the sanitizer tests).
- Its options (enabled / locale / reference clock / tooltip formatter) are read through a closure
  per call, exactly like the blocked-image label, because the pipeline itself is cached per
  remote-content mode.

Styling lives in `MENTION_CSS`, injected into the shadow body next to `THEME_LINK_CSS`. Colors are
`!important` for the same reason as `.email-auto-link`: the message's own CSS may style `span`
broadly. Text-only messages get the same chips via `linkifyPlainText(text, mentions)`.

`EmailDetail.handleLinkClick` is the single click entry point: it scans `event.composedPath()` for
`[data-remind-at]` first (a chip inside a link still counts as a chip), then falls back to the
existing external-link dialog.

## Storage

Reminders are real rows now:

- `services/reminderservice.go` — `SaveReminder` (assigns an ID when the caller sends an empty one),
  `LoadReminders` (soonest first), `DeleteReminder`, `SetReminderDone`.
- `CachedReminder` in `services/database.go` — `time` is **epoch seconds**, the project-wide
  convention and exactly what the parser produces.
- `frontend/src/stores/reminders.ts` — one list shared by the reminder page and the mail dialog;
  `toggle` updates locally first and rolls back if the save fails.
- `frontend/src/views/ReminderView.vue` uses the store (it used to run on placeholder data).

## Settings

`detect_dates_in_mail` (settings ▸ 通用 ▸ 识别邮件中的日期时间) defaults to **on**, and is read in
`EmailDetail.sanitizeOptions()` so turning it off removes the chips on the next render (the
sanitized body is a computed that depends on the preference).

Two config details worth remembering:

- `readConfig()` unmarshals **on top of `defaultConfig()`**, so a key that is absent from an older
  config file keeps its documented default instead of the Go zero value.
- Flags whose default is `true` (`scan_external_safety`, `detect_dates_in_mail`) must not use
  `omitempty`: otherwise saving "off" omits the key and the next read brings the default back.
  `TestDefaultOnFlagsPersistBeingTurnedOff` guards both.
