import { unified } from 'unified'
import rehypeParse from 'rehype-parse'
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize'
import rehypeStringify from 'rehype-stringify'
import type { Schema } from 'hast-util-sanitize'
import type { Root, Element, Text, Comment, Doctype } from 'hast'

type PropertyDefinition = [string, ...(string | number | boolean | RegExp | null | undefined)[]]

/** Tags whose text content should not be auto-linked. */
const NO_LINK_TAGS = new Set(['a', 'style', 'script', 'pre', 'code', 'noscript'])

/** Regex matching common URLs (http/https/www) and email addresses. */
const URL_RE = /(https?:\/\/[^\s<>"']+|(?:www\.)[^\s<>"']+)|([\w.+-]+@[\w-]+\.[\w.-]+)/gi

/** Keep links inside the email shadow DOM aligned with the Naive UI theme. */
const THEME_LINK_CSS = `
.email-auto-link { color: var(--n-primary-color, #18a058) !important; }
`

function rehypeAutoLink() {
  return (tree: Root) => walkAndLink(tree)
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

/** Convert plain text to safe HTML with clickable URLs. */
function linkifyPlainText(text: string): string {
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

  const stringifier = unified().use(rehypeStringify)
  return `<style>${THEME_LINK_CSS}</style>` + String(stringifier.stringify(tree))
}

const emailSchema: Schema = {
  ...defaultSchema,
  tagNames: [...(defaultSchema.tagNames ?? []), 'style'],
  attributes: {
    ...defaultSchema.attributes,
    '*': [...(defaultSchema.attributes?.['*'] ?? []), ['style'] as PropertyDefinition],
    a: [
      ...(defaultSchema.attributes?.a ?? []),
      ['className', 'email-auto-link'] as PropertyDefinition,
    ],
  },
  protocols: {
    ...defaultSchema.protocols,
    src: [
      ...((defaultSchema.protocols as Record<string, string[]> | undefined)?.['src'] ?? []),
      'data',
      'cid',
    ],
  },
}

/** Visually mark images whose source was removed by remote-content blocking. */
const BLOCKED_IMAGE_CSS = `
img[data-remote-blocked] { min-width: 14px; min-height: 14px; outline: 1px dashed rgba(128, 128, 128, 0.45); }
`

/** Absolute URL (http/https or protocol-relative), i.e. something fetched off-device. */
const REMOTE_URL_RE = /^(?:https?:)?\/\//i

/**
 * Attributes that make a browser fetch a resource. Both spellings of `srcset`
 * are listed on purpose: hast stores property names camel-cased (`srcSet`), and
 * a missed spelling would leave `<picture><source srcset=…>` able to load a
 * remote tracking image.
 */
const URL_ATTRS = ['src', 'srcSet', 'srcset', 'poster', 'background', 'data'] as const

/** True when any candidate URL inside the attribute value points off-device. */
function hasRemoteRef(value: string): boolean {
  // srcset is a comma-separated list of "url descriptor" candidates.
  return value.split(',').some((candidate) => REMOTE_URL_RE.test(candidate.trim().split(/\s+/)[0] ?? ''))
}

/**
 * Sanitize one CSS text (a `<style>` block or a `style=` attribute value).
 *
 * This is a pragmatic blocklist, not a full CSS parser: it removes the
 * constructs that would let message CSS fetch remote resources or escape the
 * message area, while leaving layout/colour declarations untouched so email
 * still renders. Remote `url()` are only stripped when remote content is
 * blocked; `data:`/`cid:` (our resolved inline images) are always kept.
 */
function sanitizeCss(css: string, allowRemote: boolean): { css: string; blocked: number } {
  let blocked = 0
  // Comments are dropped first: they can split the tokens matched below.
  let out = css.replace(/\/\*[\s\S]*?\*\//g, ' ')

  // @import pulls an arbitrary external stylesheet — never allowed.
  out = out.replace(/@import[^;]*;?/gi, () => {
    blocked++
    return ''
  })

  if (!allowRemote) {
    out = out.replace(/url\(\s*(['"]?)([^'")]*)\1\s*\)/gi, (whole: string, _q: string, target: string) => {
      if (REMOTE_URL_RE.test(String(target).trim())) {
        blocked++
        return 'none'
      }
      return whole
    })
  }

  // Legacy code-execution / binding vectors, and layout that could overlay the app.
  out = out.replace(/(?:-moz-binding|behavior)\s*:[^;}]*;?/gi, '')
  out = out.replace(/expression\s*\([^)]*\)/gi, '')
  out = out.replace(/position\s*:\s*(?:fixed|sticky)/gi, 'position: static')

  return { css: out, blocked }
}

function walkElements(node: Root | Element, visit: (el: Element) => void): void {
  if (!('children' in node)) return
  for (const child of node.children) {
    if (child.type !== 'element') continue
    visit(child)
    walkElements(child, visit)
  }
}

/** Apply `sanitizeCss` to every `<style>` block and `style=` attribute. */
function rehypeSanitizeEmailCss(options: { allowRemote: boolean; onBlocked: (n: number) => void }) {
  return (tree: Root) => {
    walkElements(tree, (el) => {
      if (el.tagName === 'style') {
        for (const child of el.children) {
          if (child.type !== 'text') continue
          const result = sanitizeCss(child.value, options.allowRemote)
          child.value = result.css
          options.onBlocked(result.blocked)
        }
        return
      }
      const style = el.properties?.['style']
      if (typeof style !== 'string') return
      const result = sanitizeCss(style, options.allowRemote)
      el.properties!['style'] = result.css
      options.onBlocked(result.blocked)
    })
  }
}

/** Drop remote resource references from URL-bearing attributes. */
function rehypeBlockRemoteContent(options: { allowRemote: boolean; onBlocked: (n: number) => void }) {
  return (tree: Root) => {
    if (options.allowRemote) return
    walkElements(tree, (el) => {
      for (const attr of URL_ATTRS) {
        const value = el.properties?.[attr]
        if (typeof value !== 'string' || !hasRemoteRef(value)) continue
        delete el.properties![attr]
        el.properties!['data-remote-blocked'] = '1'
        options.onBlocked(1)
      }
    })
  }
}

export interface EmailSanitizeOptions {
  /** Load remote images/fonts/imported CSS. Default false — remote content is blocked. */
  allowRemote?: boolean
}

export interface EmailSanitizeResult {
  html: string
  /** Remote resources that were removed (image sources plus CSS references). */
  blockedRemote: number
}

// Two reusable pipelines (blocking / allowing remote content); building them is
// the expensive part, so they are cached per mode.
const sanitizers = new Map<boolean, ReturnType<typeof createSanitizer>>()

function createSanitizer(allowRemote: boolean) {
  let blockedRemote = 0
  // The counter drives the "remote content was blocked" banner, so only count in
  // blocking mode: after the user allows remote content there is nothing left to
  // offer, and a stale count would make the banner reappear.
  const onBlocked = allowRemote
    ? () => {}
    : (n: number) => {
        blockedRemote += n
      }
  const processor = unified()
    .use(rehypeParse, { fragment: true })
    .use(rehypeAutoLink)
    .use(rehypeSanitize, emailSchema)
    // Both run after sanitising, so they only rewrite/remove what it kept.
    .use(rehypeSanitizeEmailCss, { allowRemote, onBlocked })
    .use(rehypeBlockRemoteContent, { allowRemote, onBlocked })
    .use(rehypeStringify)
  return { processor, takeBlocked: () => {
    const n = blockedRemote
    blockedRemote = 0
    return n
  } }
}

/**
 * Sanitize an email body: strips dangerous markup, neutralises unsafe CSS and
 * (unless `allowRemote`) removes remote images/styles and reports how many were
 * blocked, so the UI can offer to load them.
 */
function sanitizeEmailHtml(html: string, options: EmailSanitizeOptions = {}): EmailSanitizeResult {
  const allowRemote = options.allowRemote === true
  let entry = sanitizers.get(allowRemote)
  if (!entry) {
    entry = createSanitizer(allowRemote)
    sanitizers.set(allowRemote, entry)
  }
  const result = entry.processor.processSync(html)
  const blockedRemote = entry.takeBlocked()
  return {
    html: String(result) + `<style>${THEME_LINK_CSS}${BLOCKED_IMAGE_CSS}</style>`,
    blockedRemote,
  }
}

/** Sanitize HTML while preserving email layout and safe inline styles. */
function sanitizeHtmlRehype(html: string, options: EmailSanitizeOptions = {}): string {
  return sanitizeEmailHtml(html, options).html
}

export { sanitizeHtmlRehype, sanitizeEmailHtml, linkifyPlainText }
