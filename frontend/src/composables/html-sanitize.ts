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

/**
 * Placeholder that replaces a blocked `<img>`.
 *
 * The element is swapped rather than restyled because an `<img>` is a replaced
 * element: WebKit does not render its pseudo-elements, so alt text cannot be
 * styled or overlaid on it. A `<span>` gives us the striped background, the
 * inline icon and the alt text in one box.
 */
const BLOCKED_IMAGE_CSS = `
.remote-blocked {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  box-sizing: border-box;
  max-width: 100%;
  max-height: 320px;
  min-height: 26px;
  padding: 4px 8px;
  overflow: hidden;
  border-radius: var(--radius-sm);
  /* !important because dark-mode adaptation injects
     \`* { background: transparent !important }\`; the class beats \`*\`. */
  background: repeating-linear-gradient(
    45deg,
    var(--blocked-stripe-a) 0 6px,
    var(--blocked-stripe-b) 6px 12px
  ) !important;
  color: var(--color-text-3);
  font-size: 12px;
  line-height: 1.4;
  text-align: center;
  overflow-wrap: anywhere;
  vertical-align: middle;
}
.remote-blocked::before {
  content: '';
  flex: 0 0 auto;
  width: 14px;
  height: 14px;
  background-color: currentColor;
  -webkit-mask-image: url('data:image/svg+xml;base64,PHN2ZyB4bWxucz0naHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmcnIHZpZXdCb3g9JzAgMCAyNCAyNCc+PHBhdGggZD0nTTQgNWgxNmExIDEgMCAwIDEgMSAxdjEyYTEgMSAwIDAgMS0xIDFINGExIDEgMCAwIDEtMS0xVjZhMSAxIDAgMCAxIDEtMVptMSAydjguNmw0LjMtNC4zIDMuMiAzLjIgMi44LTIuOEwxOSAxNS40VjdINVptMy41IDFhMS41IDEuNSAwIDEgMSAwIDMgMS41IDEuNSAwIDAgMSAwLTNaJy8+PC9zdmc+');
  -webkit-mask-repeat: no-repeat;
  -webkit-mask-position: center;
  -webkit-mask-size: contain;
  mask-image: url('data:image/svg+xml;base64,PHN2ZyB4bWxucz0naHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmcnIHZpZXdCb3g9JzAgMCAyNCAyNCc+PHBhdGggZD0nTTQgNWgxNmExIDEgMCAwIDEgMSAxdjEyYTEgMSAwIDAgMS0xIDFINGExIDEgMCAwIDEtMS0xVjZhMSAxIDAgMCAxIDEtMVptMSAydjguNmw0LjMtNC4zIDMuMiAzLjIgMi44LTIuOEwxOSAxNS40VjdINVptMy41IDFhMS41IDEuNSAwIDEgMSAwIDMgMS41IDEuNSAwIDAgMSAwLTNaJy8+PC9zdmc+');
  mask-repeat: no-repeat;
  mask-position: center;
  mask-size: contain;
}
/* Tracking pixels: the sender's own box, stripes only (no icon). */
.remote-blocked--dot {
  min-height: 0;
  padding: 0;
}
/* Too small for text: the icon fills the box. */
.remote-blocked--icon {
  min-height: 0;
  padding: 3px;
}
.remote-blocked--icon::before {
  width: 12px;
  height: 12px;
  max-width: 100%;
  max-height: 100%;
}
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

/** Number of URL attributes of this element that point off-device. */
function countRemoteRefs(el: Element): number {
  let count = 0
  for (const attr of URL_ATTRS) {
    const value = el.properties?.[attr]
    if (typeof value === 'string' && hasRemoteRef(value)) count++
  }
  return count
}

/** Strip remote URL attributes in place (non-image elements). */
function stripRemoteRefs(el: Element, onBlocked: (n: number) => void): void {
  for (const attr of URL_ATTRS) {
    const value = el.properties?.[attr]
    if (typeof value !== 'string' || !hasRemoteRef(value)) continue
    delete el.properties![attr]
    el.properties!['data-remote-blocked'] = '1'
    onBlocked(1)
  }
}

/**
 * Build the visible stand-in for a blocked image: it keeps the alt text (so the
 * message still reads correctly) and any sender-declared box, but shows our
 * striped background instead of a broken-image icon.
 */
function blockedImagePlaceholder(img: Element, label: string): Element {
  const properties = img.properties ?? {}
  const alt = typeof properties.alt === 'string' ? properties.alt.trim() : ''
  const width = typeof properties.width === 'number' ? properties.width : 0
  const height = typeof properties.height === 'number' ? properties.height : 0
  const maxSide = Math.max(width, height)

  const classes = ['remote-blocked']
  if (maxSide > 0 && maxSide <= 2) {
    // A tracking pixel: keep its box (usually 1×1) so the layout is untouched.
    classes.push('remote-blocked--dot')
  } else if (!alt || (maxSide > 0 && maxSide <= 24)) {
    classes.push('remote-blocked--icon')
  }

  // The placeholder always takes the image's own declared box (a 1×1 tracker
  // stays 1×1, a 16×16 logo stays 16×16); CSS caps both dimensions so a bogus
  // size cannot blow the layout up. With no declared size the box hugs its
  // content (alt text, or just the icon).
  const style = width > 0 || height > 0
    ? `width:${width > 0 ? `${width}px` : 'auto'};height:${height > 0 ? `${height}px` : 'auto'}`
    : undefined

  return {
    type: 'element',
    tagName: 'span',
    properties: {
      className: classes,
      'data-remote-blocked': '1',
      ...(label ? { title: label } : {}),
      ...(style ? { style } : {}),
    },
    // Empty alt => the CSS ::before icon is the only content.
    children: alt ? [{ type: 'text', value: alt }] : [],
  }
}

/**
 * Drop remote resource references. Blocked `<img>` elements are replaced by a
 * styled placeholder (see BLOCKED_IMAGE_CSS); other elements just lose the
 * attribute, and counting drives the "N remote resources blocked" banner.
 */
function rehypeBlockRemoteContent(options: {
  allowRemote: boolean
  onBlocked: (n: number) => void
  imageLabel: () => string
}) {
  const walk = (node: Root | Element) => {
    const children = node.children
    if (!children) return
    for (let i = 0; i < children.length; i++) {
      const child = children[i]
      if (child.type !== 'element') continue

      const remoteRefs = countRemoteRefs(child)
      if (child.tagName === 'img' && remoteRefs > 0) {
        options.onBlocked(remoteRefs)
        children[i] = blockedImagePlaceholder(child, options.imageLabel())
        continue
      }
      stripRemoteRefs(child, options.onBlocked)
      walk(child)
    }
  }

  return (tree: Root) => {
    if (options.allowRemote) return
    walk(tree)
  }
}

export interface EmailSanitizeOptions {
  /** Load remote images/fonts/imported CSS. Default false — remote content is blocked. */
  allowRemote?: boolean
  /** Tooltip for blocked-image placeholders (translated by the caller). */
  blockedImageLabel?: string
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
  // The placeholder tooltip is per call (it is translated), so the cached
  // pipeline reads it through a closure instead of baking it in.
  let imageLabel = ''
  const processor = unified()
    .use(rehypeParse, { fragment: true })
    .use(rehypeAutoLink)
    .use(rehypeSanitize, emailSchema)
    // Both run after sanitising, so they only rewrite/remove what it kept.
    .use(rehypeSanitizeEmailCss, { allowRemote, onBlocked })
    .use(rehypeBlockRemoteContent, { allowRemote, onBlocked, imageLabel: () => imageLabel })
    .use(rehypeStringify)
  return {
    processor,
    setImageLabel: (label: string) => {
      imageLabel = label
    },
    takeBlocked: () => {
      const n = blockedRemote
      blockedRemote = 0
      return n
    },
  }
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
  entry.setImageLabel(options.blockedImageLabel ?? '')
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
