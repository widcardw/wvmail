import { unified } from 'unified'
import rehypeStringify from 'rehype-stringify'
import type { Root, Element, Text, Comment, Doctype } from 'hast'
// Explicit `.ts` so the Node-run tests can resolve it (see tsconfig).
import { findDateMentions } from '../date-mentions/index.ts'
import type { DateMention } from '../date-mentions/index.ts'
import { CONTENT_FIT_CSS } from './content-fit.ts'

// Turning text into links and date/time chips: the rehype passes for auto-linking
// and for highlighting detected moments, plus the plain-text entry point.

/** Tags whose text content should not be auto-linked. */
const NO_LINK_TAGS = new Set(['a', 'style', 'script', 'pre', 'code', 'noscript'])

/** Regex matching common URLs (http/https/www) and email addresses. */
const URL_RE = /(https?:\/\/[^\s<>"']+|(?:www\.)[^\s<>"']+)|([\w.+-]+@[\w-]+\.[\w.-]+)/gi

/** Keep links inside the email shadow DOM aligned with the Naive UI theme. */
export const THEME_LINK_CSS = `
.email-auto-link { color: var(--n-primary-color, #e11d48) !important; }
`

/** Class marking a detected date/time mention (also used to skip our own output). */
const MENTION_CLASS = 'date-mention'

/**
 * Highlight for detected date/time mentions. The email's own CSS may style `span`
 * broadly, so the two visual properties are forced — same reasoning as `.email-auto-link`.
 */

export const MENTION_CSS = `
.${MENTION_CLASS} {
  padding: 0 1px;
  cursor: pointer;
  text-decoration: underline dotted;
  text-underline-offset: 2px;
  transition: background 0.2s ease-in-out, color 0.2s ease-in-out;
}
.${MENTION_CLASS}:hover {
  color: var(--n-primary-color, #e11d48) !important;
  background: color-mix(in srgb, var(--n-primary-color, #e11d48) 14%, transparent) !important;
  border-radius: 3px;
}
`

export function rehypeAutoLink() {
  return (tree: Root) => walkAndLink(tree)
}

/** Options the date/time highlighter needs, read fresh on every sanitize call. */
export interface MentionRenderOptions {
  /** When false the pass is a no-op (the settings toggle). */
  enabled: boolean
  /** App locale, so `3/5`-style ambiguity follows the UI language. */
  locale: string
  /** Reference "now" for relative mentions (`明天`, `tomorrow`). */
  now: Date
  /** Tooltip for a mention — translated by the caller. */
  label?: (at: Date) => string
}

/**
 * Elements whose text is never scanned for date/time mentions.
 *
 * `pre`/`code` are deliberately NOT here (unlike the auto-link pass): plenty of real
 * mail — every Microsoft CMT notification, for one — is plain text wrapped in a
 * single `<pre>`, so skipping it silently disabled the feature for whole senders.
 * A chip inside a code snippet is harmless noise next to that.
 */

const NO_MENTION_TAGS = new Set(['a', 'style', 'script', 'noscript'])

/**
 * Wrap detected date/time mentions in a `<span class="date-mention">` carrying the
 * resolved moment, so the mail view can offer "add to reminders" on click. Runs
 * after sanitising (like the CSS/remote passes), so its own markup never has to
 * survive the schema — and email markup can never fake a mention.
 *
 * Note this runs on `body_html` even when that is really plain text (CMT and friends
 * ship `<pre>`-wrapped text): the parser turns it into text nodes like any other body.
 */

export function rehypeDateMentions(read: () => MentionRenderOptions) {
  return (tree: Root) => {
    const options = read()
    if (!options.enabled) return
    walkDateMentions(tree, options)
  }
}

function walkDateMentions(node: Root | Element, options: MentionRenderOptions): void {
  if (!('children' in node)) return
  if (node.type === 'element' && NO_MENTION_TAGS.has(node.tagName)) return

  const newChildren: (Text | Element | Comment | Doctype)[] = []
  let changed = false

  for (const child of node.children) {
    if (child.type === 'text') {
      const mentions = findDateMentions(child.value, {
        now: options.now,
        locale: options.locale,
      })
      if (mentions.length === 0) {
        newChildren.push(child)
        continue
      }
      newChildren.push(...mentionNodes(child.value, mentions, options))
      changed = true
    } else {
      // Never re-wrap our own output (guards a second sanitize of the same tree).
      if (child.type === 'element' && !isMentionElement(child)) {
        walkDateMentions(child, options)
      }
      newChildren.push(child)
    }
  }

  if (changed) node.children = newChildren
}

function isMentionElement(node: Element): boolean {
  const className = node.properties?.className
  return Array.isArray(className) && className.includes(MENTION_CLASS)
}

function mentionNodes(
  text: string,
  mentions: DateMention[],
  options: MentionRenderOptions,
): (Text | Element)[] {
  const result: (Text | Element)[] = []
  let cursor = 0

  for (const mention of mentions) {
    if (mention.index > cursor) {
      result.push({ type: 'text', value: text.slice(cursor, mention.index) })
    }
    const properties: Record<string, string | string[]> = {
      className: [MENTION_CLASS],
      // Epoch seconds: what the click handler forwards to the reminder store.
      dataRemindAt: String(mention.epoch),
      dataRemindText: mention.text,
    }
    // Only set when the clock time was actually re-read in that zone — a zone named next
    // to a bare date (`September 15, 2026 AOE`) is part of the chip but nothing moved.
    if (mention.zone && mention.zoneApplied) properties.dataRemindZone = mention.zone
    if (options.label) properties.title = options.label(mention.at)
    result.push({
      type: 'element',
      tagName: 'span',
      properties,
      children: [{ type: 'text', value: mention.text }],
    })
    cursor = mention.index + mention.length
  }

  if (cursor < text.length) {
    result.push({ type: 'text', value: text.slice(cursor) })
  }
  return result
}

function walkAndLink(node: Root | Element): void {
  if (!('children' in node)) return
  if (node.type === 'element' && NO_LINK_TAGS.has(node.tagName)) return

  const newChildren: (Text | Element | Comment | Doctype)[] = []
  let changed = false

  for (const child of node.children) {
    if (child.type === 'text') {
      const parts = autoLinkText(child.value)
      if (parts.length === 1 && parts[0] === child) {
        newChildren.push(child)
      } else {
        newChildren.push(...parts)
        changed = true
      }
    } else {
      if (child.type === 'element') walkAndLink(child)
      newChildren.push(child)
    }
  }

  if (changed) node.children = newChildren
}

function autoLinkText(text: string): (Text | Element | Comment)[] {
  const result: (Text | Element | Comment)[] = []
  let lastIndex = 0
  let match: RegExpExecArray | null

  URL_RE.lastIndex = 0

  while ((match = URL_RE.exec(text)) !== null) {
    const fullMatch = match[0]
    const httpOrWwwUrl = match[1]
    const emailAddr = match[2]
    const isEmail = httpOrWwwUrl == null && !!emailAddr

    if (match.index > lastIndex) {
      result.push({ type: 'text', value: text.slice(lastIndex, match.index) })
    }

    let href: string
    let display: string
    if (isEmail) {
      href = `mailto:${emailAddr}`
      display = emailAddr!
    } else if (httpOrWwwUrl!.startsWith('www.')) {
      href = `https://${httpOrWwwUrl}`
      display = httpOrWwwUrl!
    } else {
      href = httpOrWwwUrl!
      display = httpOrWwwUrl!
    }

    result.push({
      type: 'element',
      tagName: 'a',
      properties: {
        href,
        target: '_blank',
        rel: 'noopener noreferrer',
        className: ['email-auto-link'],
      },
      children: [{ type: 'text', value: display }],
    })

    lastIndex = match.index + fullMatch.length
  }

  if (lastIndex < text.length) {
    result.push({ type: 'text', value: text.slice(lastIndex) })
  }

  return result.length > 0 ? result : [{ type: 'text', value: text }]
}

/** Convert plain text to safe HTML with clickable URLs and date/time mentions. */
export function linkifyPlainText(text: string, mentions?: MentionRenderOptions): string {
  const tree: Root = {
    type: 'root',
    children: [
      {
        type: 'element',
        tagName: 'div',
        properties: { style: 'white-space: pre-wrap' },
        children: autoLinkText(text),
      },
    ],
  }

  if (mentions?.enabled) walkDateMentions(tree, mentions)

  const stringifier = unified().use(rehypeStringify)
  return (
    `<style>${THEME_LINK_CSS}${CONTENT_FIT_CSS}${mentions?.enabled ? MENTION_CSS : ''}</style>` +
    String(stringifier.stringify(tree))
  )
}
