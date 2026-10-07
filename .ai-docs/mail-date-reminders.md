---
description: Detecting date/time expressions inside email bodies, highlighting them in the shadow body, and turning a click into a reminder (chrono-node parser, RemindersService, settings)
---

# Mail dates → reminders

A message that says `3月5日下午3点 评审` or `Board call Mar 5, 2026 at 3:30 PM` renders those
moments as clickable chips. Clicking one opens a confirmation dialog prefilled with the mail's
subject and the resolved time; confirming stores a reminder. When the detected moment is already in
the past the dialog says so (that chip can still be kept as a note, but it will not fire).

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

`frontend/src/composables/date-mentions/index.ts` answers "what moments are in this text?" by running
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
- **Plausibility window, measured against the message.** A moment may sit up to a year *before* the
  message's own date and up to five years after it; only long-past context is dropped. The window is
  deliberately wide because deadlines are routinely mentioned after the fact ("The review deadline
  has been extended to September 28, 2026, at 23:59 AoE (UTC-12)" arrives days later), and because
  judging "past" against the wall clock hides every timestamp in an old message — that bug is also
  why `2026-09-25 11:28:04 UTC+08:00` used to be reduced to its `08:00` fragment: the full match
  looked "past" while the offset tail rolled forward to tomorrow. The constant is `PAST_GRACE_MS` in
  `composables/date-mentions/index.ts` if the noise ever outweighs the misses.
- **One chip per moment.** Overlapping candidates collapse: most certain fields, then longest text,
  then the UI language's parser.
- **Halves of one expression are glued back together.** chrono returns `by 3 Oct (23:59 PST)` as two
  results — the bracket stops it joining the day to the clock time, and the second half even swallows
  the stray `)` — so neighbouring *complementary* mentions (one with a day and no time, one with a
  time and no day) separated only by punctuation/brackets/whitespace or the connectives `at`/`on` are
  re-read as a single expression with the brackets blanked out (same length, so offsets still map).
  The joined text must parse as exactly one day+time mention, otherwise both halves stay as they
  were. That is also what makes `3 Oct 23:59 PST` resolve on October 3rd instead of "tomorrow", and
  it fixes `3:30 PM on Mar 5` counting as two chips.

Deliberately **out of scope** (say the word before reaching for more regex): ranges
(`3月5-7日`), durations (`两周后`), recurrence, lunar dates.

**Timezones are applied** (see below), because a deadline printed in another zone is off by most of a
day if you read the clock time as local. chrono-node handles a few of
those already, but we do not promise them.

### Timezones

The clock time a message prints may belong to another zone (`23:59 AoE`, `15:00 CET`, `(UTC+05:30)`).
Left alone, that instant lands hours off — for `AoE` it is a full 20 hours in UTC+8. Two mechanisms
cover it:

- **chrono handles what it knows** (`UTC+08:00` glued to the time, `CET`, even `(UTC+05:30)`) and
  returns the correct instant; we only read the trailing zone expression back out so the dialog can
  label it.
- **Everything else goes through `ZONE_OFFSETS`** — a curated table of the names chrono does not know
  (`aoe` = UTC-12, plus PT/ET/CT/MT, JST/KST/HKT/SGT/IST, AEST/NZST … ). After chrono returns the
  clock time read as *local*, the candidate is shifted by `localOffset - zoneOffset`.

Rules that keep it predictable:

- An **explicit offset in the text always wins** over the table, so `AoE (UTC-12)` works even if the
  name were unknown.
- **Two-letter names must be uppercase** (`ET`, not `et`): chrono otherwise reads "15:30 et al" as
  Eastern Time, and French "15:00 et 16:00" would inherit a bogus offset. A match whose zone token is
  a lowercase two-letter word is dropped.
- `CST` is ambiguous on purpose-resolved: China Standard Time (+8) in a zh UI, US Central (-6)
  otherwise (`AMBIGUOUS_ZONES`).
- The chip covers the whole expression (`September 28, 2026, at 23:59 AoE (UTC-12)`), and the dialog
  says *"原文写的是 AoE (UTC-12)，已换算成本地时间"* so a converted time never looks like a typo.
- **A dangling zone is absorbed too** (`absorbTrailingZone`). chrono happily stops short of it when the
  expression is `September 15, 2026 AOE` (bare date) or `September 25, 2026 (23:59) (PST)` (the zone
  sits past the bracket the merge closed), so after merging we look for a zone expression right after
  each final mention and pull it into the chip. A mention with a clock time is converted; a **bare
  date is not** — there is no instant to move and shifting noon would silently change the day — it
  just carries the label, with `zoneApplied = false` so the dialog does not claim a conversion.

The table is a curated shortlist rather than an exhaustive one: a wrong offset is visible in the
dialog, but a long tail of ambiguous abbreviations would make that noise constant.

**The reference moment is the message's date, not "now".** `EmailDetail.sanitizeOptions()` passes
`detail.date` (falling back to the clock when it is missing), so relative words follow the message
("明天" in a March mail lands on March 2nd), the plausibility window moves with it, and `forwardDate`
resolves a bare `9:05` against that morning.

`frontend/tests/date-mentions.test.mts` is table-driven with a fixed reference date
(2026-03-01 10:00, a Sunday): **a new format is one new row**, not a new branch. To check a string
from a real message:

```bash
cd frontend && node --input-type=module -e "
import { findDateMentions } from './src/composables/date-mentions/index.ts'
console.log(findDateMentions(process.argv[1], { locale: 'zh-CN' }))" '待测文本'
```

## Highlighting

`rehypeDateMentions` in `frontend/src/composables/html/sanitize.ts` walks text nodes (skipping
`a`/`code`/`pre`/`style`/`script`, and never re-wrapping its own output) and replaces each match with:

```html
<span class="date-mention" data-remind-at="1772694000" data-remind-text="3月5日下午3点"
      title="点击加入提醒事项：…">3月5日下午3点</span>
```

Two properties of this pass matter:

- It runs **after** `rehypeSanitize`, like the CSS and remote-content passes. The schema therefore
  needs no `span`/`data-*` allowance, and an email cannot fake a chip: `class="date-mention"` and
  `data-remind-at` are stripped from the message's own markup (covered by the sanitizer tests).
- The walker skips `a`, `style`, `script` and `noscript` — but **not** `pre`/`code`. The auto-link pass
  skips those two to avoid mangling code samples; here that would have killed the feature for whole
  senders, because plenty of real mail is plain text wrapped in a single `<pre>` (every Microsoft CMT
  notification: `<!DOCTYPE html>…<body><pre style="font-family: monospace">…`). A chip inside a code
  snippet is the cheaper mistake.
- Its options (enabled / locale / reference clock / tooltip formatter) are read through a closure
  per call, exactly like the blocked-image label, because the pipeline itself is cached per
  remote-content mode.

Styling lives in `MENTION_CSS`, injected into the shadow body next to `THEME_LINK_CSS`. Colors are
`!important` for the same reason as `.email-auto-link`: the message's own CSS may style `span`
broadly. Text-only messages get the same chips via `linkifyPlainText(text, mentions)`.

`EmailDetail.handleLinkClick` is the single click entry point: it scans `event.composedPath()` for
`[data-remind-at]` first (a chip inside a link still counts as a chip), then falls back to the
existing external-link dialog.

### Debugging a message that was not detected

Read the cached body straight out of SQLite and run the detector on it (read-only query):

```bash
DB="$HOME/Library/Application Support/wvmail/cache.db"
sqlite3 -readonly "$DB" "SELECT uid, subject, date, length(body_text), length(body_html) FROM cached_email_bodies WHERE subject LIKE '%<keyword>%';"
sqlite3 -readonly "$DB" "SELECT body_html FROM cached_email_bodies WHERE uid=<uid>;" > /tmp/mail.html
cd frontend && node --input-type=module -e "
import { readFileSync } from 'node:fs'
import { sanitizeEmailHtml } from './src/composables/html/sanitize.ts'
const html = readFileSync('/tmp/mail.html', 'utf8')
const out = sanitizeEmailHtml(html, { mentions: { enabled: true, locale: 'zh-CN', now: new Date('<mail date>') } })
console.log(out.html.match(/<span class=\"date-mention\"[^>]*>[^<]*<\/span>/g) ?? 'no chips')
"
```

`body_html` being plain text is normal (see the `pre` note above) — the definitive check is whether
`findDateMentions(rawText, { now: mailDate })` finds it and `sanitizeEmailHtml` then drops it.

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
