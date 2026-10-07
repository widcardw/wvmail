/**
 * Regression tests for note text extraction (`composables/note-text.ts`: the sidebar
 * title, the row preview and the row date).
 *
 * Run with: pnpm test   (Node ≥ 23 strips TypeScript types natively)
 *
 * The old inline implementations stripped tags with one regex over the raw HTML,
 * which glued blocks together (`<p>a</p><p>b</p>` → `ab`) and left entities visible
 * (`&amp;`). The cases below pin the differences down, including the one that is easy
 * to get backwards: an *escaped* tag in the note must survive as text, so tags are
 * stripped before entities are decoded.
 */
import {
  decodeNoteEntities,
  extractNoteTitle,
  formatNoteDate,
  noteHtmlToText,
  notePreviewText,
} from '../src/composables/note-text.ts'

let pass = 0
let fail = 0

function check(name: string, cond: boolean) {
  if (cond) {
    pass++
    console.log(`  PASS  ${name}`)
  } else {
    fail++
    console.error(`  FAIL  ${name}`)
  }
}

console.log('note text')

check('blocks separate with newlines', noteHtmlToText('<p>a</p><p>b</p>') === 'a\nb')
check('a line break is a break', noteHtmlToText('a<br>b') === 'a\nb')
check('list items are separate lines', noteHtmlToText('<ul><li>one</li><li>two</li></ul>') === 'one\ntwo')
check('headings break too', noteHtmlToText('<h1>Title</h1><p>body</p>') === 'Title\nbody')
check('inline markup does not break', noteHtmlToText('<p>a <strong>b</strong> c</p>') === 'a b c')
check('an empty note is empty text', noteHtmlToText('') === '')
check('empty paragraphs disappear', noteHtmlToText('<p></p><p>real</p>') === 'real')
check('nbsp becomes a space, not a line', noteHtmlToText('<p>a&nbsp;b</p>') === 'a b')
check('a space separator collapses blocks', noteHtmlToText('<p>a</p><p>b</p>', ' ') === 'a b')

check('entities decode', decodeNoteEntities('a &amp; b &lt;c&gt; &quot;d&quot;') === 'a & b <c> "d"')
check('numeric entities decode', decodeNoteEntities('&#65;&#x42;') === 'AB')
check('an unknown entity is left alone', decodeNoteEntities('&frobnicate;') === '&frobnicate;')
check('an escaped tag survives as text', noteHtmlToText('<p>&lt;b&gt;bold?&lt;/b&gt;</p>') === '<b>bold?</b>')
check('a real tag does not', noteHtmlToText('<p><b>bold</b></p>') === 'bold')

check('the title is the first line', extractNoteTitle('<p>Hello</p><p>world</p>', 'Untitled') === 'Hello')
check('the title is capped at 15 chars', extractNoteTitle(`<p>${'x'.repeat(40)}</p>`, 'Untitled') === 'x'.repeat(15))
check('an empty note falls back to the label', extractNoteTitle('<p></p>', '无标题') === '无标题')
check('an empty string falls back too', extractNoteTitle('', 'Untitled') === 'Untitled')
check('a whitespace-only first line is skipped', extractNoteTitle('<p> </p><p>second</p>', 'Untitled') === 'second')

const long = `<p>${'y'.repeat(80)}</p>`
check('the preview is capped at 60 chars', notePreviewText(long, 'empty').length === 61)
check('…and marked as truncated', notePreviewText(long, 'empty').endsWith('…'))
check('a short note is shown whole', notePreviewText('<p>short</p>', 'empty') === 'short')
check('an empty note shows the empty label', notePreviewText('<p></p>', '暂无笔记') === '暂无笔记')

const now = new Date('2026-03-05T15:30:00')
check('no timestamp shows nothing', formatNoteDate(0, now) === '')
check(
  'a note from today shows the time',
  formatNoteDate(Math.floor(new Date('2026-03-05T09:05:00').getTime() / 1000), now).length > 0,
)
check(
  'a note from this year shows month and day',
  formatNoteDate(Math.floor(new Date('2026-01-20T09:05:00').getTime() / 1000), now).includes('20'),
)
check(
  'an older note includes the year',
  formatNoteDate(Math.floor(new Date('2024-01-20T09:05:00').getTime() / 1000), now).includes('2024'),
)

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
