import { unified } from 'unified'
import rehypeParse from 'rehype-parse'
import rehypeSanitize, { defaultSchema } from 'rehype-sanitize'
import rehypeStringify from 'rehype-stringify'
import type { Schema } from 'hast-util-sanitize'
import type { Root, Element, Text } from 'hast'

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
      if (child.type === 'element') walkAndLink(child)
      newChildren.push(child)
    }
  }

  if (changed) node.children = newChildren
}

function autoLinkText(text: string): (Text | Element)[] {
  const result: (Text | Element)[] = []
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
    '*': [...((defaultSchema.attributes as Record<string, unknown[]>)?.['*'] ?? []), ['style']],
    a: [
      ...((defaultSchema.attributes as Record<string, unknown[]>)?.a ?? []),
      ['className', 'email-auto-link'],
    ],
  },
  protocols: {
    ...defaultSchema.protocols,
    src: [...((defaultSchema.protocols as Record<string, string[]> | undefined)?.['src'] ?? []), 'data', 'cid'],
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

/** Sanitize HTML while preserving email layout and safe inline styles. */
function sanitizeHtmlRehype(html: string): string {
  if (!sanitizer) sanitizer = createSanitizer()
  const result = sanitizer.processSync(html)
  return String(result) + `<style>${THEME_LINK_CSS}</style>`
}

export { sanitizeHtmlRehype, linkifyPlainText }
