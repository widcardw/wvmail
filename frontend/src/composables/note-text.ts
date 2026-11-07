/**
 * Turning stored note HTML into the plain text the UI shows: the sidebar title, the
 * row preview, and the date next to them.
 *
 * Pure functions, so they can be tested on bare Node — see tests/note-text.test.mts.
 * The previous inline versions stripped tags with a single regex, which concatenated
 * block text (`<p>a</p><p>b</p>` → `ab`) and left entities visible (`&amp;`); the
 * order below is deliberate: strip tags **first**, decode entities **after**, so an
 * escaped tag in the note (`&lt;b&gt;`) survives as text instead of being eaten.
 */

/** Tags whose end implies a line/paragraph break in the source text. */
const BLOCK_BREAK_RE = /<\/(?:p|div|h[1-6]|li|blockquote|pre|tr)>|<br\s*\/?>/gi

const NAMED_ENTITIES: Record<string, string> = {
  amp: '&',
  lt: '<',
  gt: '>',
  quot: '"',
  apos: "'",
  nbsp: ' ',
  mdash: '—',
  ndash: '–',
  hellip: '…',
  ldquo: '“',
  rdquo: '”',
  lsquo: '‘',
  rsquo: '’',
  times: '×',
  middot: '·',
}

/** Decode the entities a rich-text editor actually emits (named + numeric). */
export function decodeNoteEntities(text: string): string {
  return text.replace(/&(#x?[0-9a-f]+|[a-z]+);/gi, (match, body: string) => {
    if (body.startsWith('#')) {
      const code =
        body[1]?.toLowerCase() === 'x' ? parseInt(body.slice(2), 16) : Number(body.slice(1))
      return Number.isFinite(code) && code > 0 ? String.fromCodePoint(code) : match
    }
    return NAMED_ENTITIES[body.toLowerCase()] ?? match
  })
}

/**
 * Note HTML → plain text. `separator` becomes the line break between blocks, so the
 * title (first line) wants `\n` and the one-line preview wants a space.
 */
export function noteHtmlToText(html: string, separator = '\n'): string {
  if (!html) return ''
  const withBreaks = html.replace(BLOCK_BREAK_RE, separator)
  const stripped = withBreaks.replace(/<[^>]*>/g, '')
  const decoded = decodeNoteEntities(stripped)
  return decoded
    .split(separator)
    .map((line) => line.replace(/[ \t\u00a0]+/g, ' ').trim())
    .filter(Boolean)
    .join(separator)
}

/** The stored title: the first line of the note, capped — or `untitled` when empty. */
export function extractNoteTitle(html: string, untitled: string, maxLen = 15): string {
  const firstLine = noteHtmlToText(html, '\n').split('\n')[0]?.trim() ?? ''
  return firstLine.slice(0, maxLen) || untitled
}

export interface NoteListPreview {
  /** First non-empty line, shown as the row's bold lead. */
  title: string
  /** A prefix of everything after that line, shown as the muted second line. */
  excerpt: string
}

/**
 * Split a note into the two lines of its sidebar row: the first non-empty line
 * becomes the title (the row's CSS ellipsis handles the length, like an email
 * subject) and the remaining lines become a one-line excerpt. A note with a
 * single line therefore has no excerpt.
 */
export function noteListPreview(
  html: string,
  untitled: string,
  excerptMaxLen = 90,
): NoteListPreview {
  const lines = noteHtmlToText(html, '\n').split('\n')
  const title = lines[0]?.trim() || untitled
  const rest = lines.slice(1).join(' ').trim()
  const excerpt = rest.length > excerptMaxLen ? `${rest.slice(0, excerptMaxLen)}…` : rest
  return { title, excerpt }
}

/** The one-line preview shown in the note list. */
export function notePreviewText(html: string, empty: string, maxLen = 60): string {
  const text = noteHtmlToText(html, ' ')
  return text.length > maxLen ? `${text.slice(0, maxLen)}…` : text || empty
}

/** Compact timestamp for a note row: time today, month/day this year, else the year. */
export function formatNoteDate(ts: number, now: Date = new Date()): string {
  if (!ts) return ''
  // Notes are stored with second-resolution timestamps; JS Date wants milliseconds.
  const date = new Date(ts * 1000)
  if (date.toDateString() === now.toDateString()) {
    return date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
  }
  if (date.getFullYear() === now.getFullYear()) {
    return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
  }
  return date.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}
