import { unified } from 'unified'
import rehypeParse from 'rehype-parse'
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize'
import rehypeStringify from 'rehype-stringify'
import { toText } from 'hast-util-to-text'
import type { Schema } from 'hast-util-sanitize'
import type { Root, Element, Text } from 'hast'

/** Tags that should not have their text content auto-linked. */
const NO_LINK_TAGS = new Set(['a', 'style', 'script', 'pre', 'code', 'noscript'])

/** Regex matching common URLs (http/https/www) and email addresses. */
const URL_RE = /(https?:\/\/[^\s<>"']+|(?:www\.)[^\s<>"']+)|([\w.+-]+@[\w-]+\.[\w.-]+)/gi

/**
 * Rehype plugin: convert plain-text URLs and email addresses into clickable `<a>` links.
 * Skips elements already inside `<a>`, `<style>`, `<script>`, `<pre>`, `<code>`.
 */
function rehypeAutoLink() {
  return (tree: Root) => {
    walk(tree)
  }
}

function walk(node: Root | Element): void {
  // Process children regardless of node type
  if (!('children' in node)) return
  if (node.type === 'element' && NO_LINK_TAGS.has(node.tagName)) return

  const newChildren: (Text | Element)[] = []
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
      if (child.type === 'element') walk(child)
      newChildren.push(child)
    }
  }

  if (changed) node.children = newChildren
}

/**
 * Split text by URL/email patterns, returning a mix of text nodes and `<a>` elements.
 */
function autoLinkText(text: string): (Text | Element)[] {
  const result: (Text | Element)[] = []
  let lastIndex = 0
  let match: RegExpExecArray | null

  URL_RE.lastIndex = 0

  while ((match = URL_RE.exec(text)) !== null) {
    const fullMatch = match[0]
    const httpOrWwwUrl = match[1] // group 1: http://... or www...
    const emailAddr = match[2]    // group 2: user@domain
    const isEmail = httpOrWwwUrl == null && !!emailAddr

    // Text before the match
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
      properties: { href, target: '_blank', rel: 'noopener noreferrer' },
      children: [{ type: 'text', value: display }],
    })

    lastIndex = match.index + fullMatch.length
  }

  if (lastIndex < text.length) {
    result.push({ type: 'text', value: text.slice(lastIndex) })
  }

  return result.length > 0 ? result : [{ type: 'text', value: text }]
}

/**
 * Custom sanitize schema based on GitHub's defaults, with additions for email HTML:
 * - Allow `<style>` blocks (many email layouts rely on embedded CSS)
 * - Allow `style` attribute on all elements
 * - Still strips `<script>`, `<iframe>`, `<object>`, event handlers, javascript: URIs
 */
const emailSchema: Schema = {
  ...defaultSchema,
  tagNames: [
    ...(defaultSchema.tagNames ?? []),
    'style',
  ],
  attributes: {
    ...defaultSchema.attributes,
    '*': [
      ...((defaultSchema.attributes as Record<string, unknown[]>)?.['*'] ?? []),
      ['style'],
    ],
  },
  protocols: {
    ...(defaultSchema.protocols ?? {}),
    src: [
      ...((defaultSchema.protocols as Record<string, string[]>)?.['src'] ?? []),
      'data',
      'cid',
    ],
  },
}

let sanitizer: ReturnType<typeof createSanitizer> | null = null

function createSanitizer() {
  return unified()
    .use(rehypeParse, { fragment: true })
    .use(rehypeAutoLink)
    .use(rehypeSanitize, emailSchema)
    .use(rehypeStringify)
}

let textExtractor: ReturnType<typeof createTextExtractor> | null = null

function createTextExtractor() {
  return unified().use(rehypeParse, { fragment: true })
}

/**
 * Sanitize HTML for email rendering.
 * Preserves embedded CSS and inline styles for proper email layout,
 * while removing dangerous elements like scripts/iframes/event handlers.
 */
function sanitizeHtmlRehype(html: string): string {
  if (!sanitizer) sanitizer = createSanitizer()
  const result = sanitizer.processSync(html)
  return String(result)
}

/**
 * Extract clean, readable text from HTML for translation purposes.
 * Uses hast-util-to-text which extracts text content from all text nodes,
 * preserving paragraph/lists structure with line breaks.
 */
function extractTextForTranslation(html: string): string {
  if (!textExtractor) textExtractor = createTextExtractor()
  const result = textExtractor.parseSync(html)
  return toText(result)
}

export { sanitizeHtmlRehype, extractTextForTranslation }
