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

/** Keep links inside the email shadow DOM aligned with the Naive UI theme. */
const THEME_LINK_CSS = `
a { color: var(--n-primary-color, #18a058) !important; }
`

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
    const emailAddr = match[2] // group 2: user@domain
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
 * Convert a text/plain email body to safe HTML with clickable URLs.
 *
 * Building the HAST tree directly keeps the original content as text nodes,
 * so characters such as `<` and `>` are escaped by rehype-stringify instead
 * of being interpreted as HTML. The wrapper preserves the line breaks that
 * were previously provided by `.body-text { white-space: pre-wrap }`.
 */
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

/**
 * Custom sanitize schema based on GitHub's defaults, with additions for email HTML:
 * - Allow `<style>` blocks (many email layouts rely on embedded CSS)
 * - Allow `style` attribute on all elements
 * - Still strips `<script>`, `<iframe>`, `<object>`, event handlers, javascript: URIs
 */
const emailSchema: Schema = {
  ...defaultSchema,
  tagNames: [...(defaultSchema.tagNames ?? []), 'style'],
  attributes: {
    ...defaultSchema.attributes,
    '*': [...((defaultSchema.attributes as Record<string, unknown[]>)?.['*'] ?? []), ['style']],
  },
  protocols: {
    ...(defaultSchema.protocols ?? {}),
    src: [...((defaultSchema.protocols as Record<string, string[]>)?.['src'] ?? []), 'data', 'cid'],
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
  // Append after the email's own styles so the app theme is used for links.
  return String(result) + `<style>${THEME_LINK_CSS}</style>`
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

const SEPARATOR = '\n————\n'

/**
 * Walk the AST and collect all text node values in order.
 * Also returns a visitor function to re-inject translated texts.
 */
function collectAndInject(tree: Root): {
  texts: string[]
  inject: (translated: string[]) => boolean
} {
  const texts: string[] = []
  const textNodes: { node: Text; parent: Element; index: number }[] = []

  function walk(node: Root | Element): void {
    if (!('children' in node)) return
    if (node.type === 'element' && NO_LINK_TAGS.has(node.tagName)) return

    const children = node.children
    for (let i = children.length - 1; i >= 0; i--) {
      const child = children[i]
      if (child.type === 'text') {
        const val = child.value.trim()
        if (val) {
          texts.push(child.value)
          textNodes.push({ node: child as Text, parent: node, index: i })
        }
      } else if (child.type === 'element') {
        walk(child)
      }
    }
  }

  walk(tree)
  // We walked in reverse; restore original order
  texts.reverse()
  textNodes.reverse()

  function inject(translated: string[]): boolean {
    if (translated.length !== textNodes.length) return false
    for (let i = 0; i < translated.length; i++) {
      textNodes[i].node.value = translated[i]
    }
    return true
  }

  return { texts, inject }
}

/**
 * Translate the HTML content while preserving its full layout structure.
 * Extracts text nodes → joins with separator → translates → splits →
 * injects back into the AST → stringifies to HTML.
 *
 * Returns the translated HTML, or `null` if the segment count didn't match
 * (caller should fall back to plain-text translation).
 *
 * @param html - Email HTML to translate
 * @param translateFn - Async function that translates a joined text block
 * @returns Translated HTML string, or null on segment mismatch
 */
async function translateHtmlWithLayout(
  html: string,
  translateFn: (text: string) => Promise<string>,
): Promise<string | null> {
  const parser = unified().use(rehypeParse, { fragment: true })
  const tree = parser.parse(html)

  const { texts, inject } = collectAndInject(tree)

  if (texts.length === 0) return null

  // Join all text segments with a unique separator
  const joined = texts.join(SEPARATOR)

  // Translate
  const translated = await translateFn(joined)

  // Split back
  const parts = translated.split(SEPARATOR)

  if (parts.length !== texts.length) return null // fallback

  // Inject back into AST
  if (!inject(parts.map((p) => p.trim()))) return null

  // Stringify back to HTML
  const stringifier = unified().use(rehypeStringify)
  const result = stringifier.stringify(tree)
  return String(result)
}

// ─── Dark mode color adaptation ───────────────────────────────────

/** Parse a CSS hex color (#RGB, #RRGGBB) to RGB tuple, or null. */
function parseHex(hex: string): [number, number, number] | null {
  const h = hex.replace(/^#/, '')
  if (h.length === 3) {
    const [r, g, b] = h
    return [parseInt(r + r, 16), parseInt(g + g, 16), parseInt(b + b, 16)]
  }
  if (h.length === 6) {
    return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)]
  }
  return null
}

/** Named CSS colors map (common subset). */
const NAMED_COLORS: Record<string, string> = {
  black: '#000000',
  silver: '#c0c0c0',
  gray: '#808080',
  grey: '#808080',
  white: '#ffffff',
  maroon: '#800000',
  red: '#ff0000',
  purple: '#800080',
  fuchsia: '#ff00ff',
  green: '#008000',
  lime: '#00ff00',
  olive: '#808000',
  yellow: '#ffff00',
  navy: '#000080',
  blue: '#0000ff',
  teal: '#008080',
  aqua: '#00ffff',
  orange: '#ffa500',
  darkgray: '#a9a9a9',
  darkgrey: '#a9a9a9',
  lightgray: '#d3d3d3',
  lightgrey: '#d3d3d3',
  darkblue: '#00008b',
  darkgreen: '#006400',
  darkred: '#8b0000',
  transparent: '',
}

/** Parse a CSS color value string to RGB tuple, or null. */
function parseColor(color: string): [number, number, number] | null {
  const s = color.trim().toLowerCase()
  if (!s) return null
  const named = NAMED_COLORS[s]
  if (named !== undefined) return named ? parseHex(named) : null
  if (s.startsWith('#')) return parseHex(s)
  const rgbMatch = s.match(/^rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/)
  if (rgbMatch) {
    return [parseInt(rgbMatch[1], 10), parseInt(rgbMatch[2], 10), parseInt(rgbMatch[3], 10)]
  }
  return null
}

/** Calculate relative luminance (0-1) from an RGB tuple. */
function luminance(rgb: [number, number, number]): number {
  const [lr, lg, lb] = rgb.map((c) => {
    const v = c / 255
    return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4)
  })
  return 0.2126 * lr + 0.7152 * lg + 0.0722 * lb
}

/** Dark-mode adaptations for common color patterns. */
function adaptColor(color: [number, number, number], isBg: boolean): string {
  const lum = luminance(color)
  if (isBg) {
    if (lum > 0.6) return `rgb(${255 - color[0]},${255 - color[1]},${255 - color[2]})`
    if (lum > 0.35) return 'rgb(45,45,45)'
    return ''
  }
  // Text: dark on potentially dark bg
  if (lum < 0.4) return 'rgb(210,210,210)'
  return ''
}

const DM_CSS = `
:host { color-scheme: dark; }
/* 已适配的 inline color/background */
[data-dm-color] { color: var(--dm-c) !important; }
[data-dm-bg] { background-color: var(--dm-b) !important; }
[data-dm-link] { color: var(--dm-l) !important; }
`

/** General reset CSS: appended AFTER email's own <style> blocks to win later */
const DM_RESET = `
/* 通用文字兜底：所有元素文字变亮，确保任何格式 (hex/rgb/named) 的深色文字都变成可读
   * 特异度 0,0,0,0，[data-dm-color] (0,1,0,0) 和 a (0,0,0,1) 可以覆盖它 */
* { color: rgb(210,210,210) !important; }
a { color: var(--n-primary-color, #66b0ff) !important; text-decoration: underline; }
/* 通用背景兜底：清除所有未被 inline 适配的背景色 */
* { background: transparent !important; }
/* 已适配的或特殊元素不参与兜底 */
[data-dm-bg] { background-color: var(--dm-b) !important; }
img { background: none !important; }
`

/**
 * Adapt email HTML for dark mode viewing.
 * Uses Dark Reader-style approach: inject CSS custom properties + data attributes
 * + !important CSS rules to override inline colors.
 */
function adaptHtmlForDarkMode(html: string): string {
  const parser = unified().use(rehypeParse, { fragment: true })
  const tree = parser.parse(html)

  function walk(node: Root | Element): void {
    if (!('children' in node)) return
    if (node.type === 'element') {
      const style = (node.properties?.style as string) ?? ''
      if (style) {
        let colorAdapt = ''
        let bgAdapt = ''

        const colorMatch = style.match(/\bcolor\s*:\s*([^;]+)/i)
        if (colorMatch) {
          const parsed = parseColor(colorMatch[1])
          if (parsed) colorAdapt = adaptColor(parsed, false)
        }

        const bgMatch = style.match(/\bbackground-color\s*:\s*([^;]+)/i)
        if (bgMatch) {
          const parsed = parseColor(bgMatch[1])
          if (parsed) bgAdapt = adaptColor(parsed, true)
        }

        const attrs: Record<string, string> = {}
        const cssVars: string[] = []

        if (colorAdapt) {
          attrs['data-dm-color'] = ''
          cssVars.push(`--dm-c: ${colorAdapt}`)
        }
        if (bgAdapt) {
          attrs['data-dm-bg'] = ''
          cssVars.push(`--dm-b: ${bgAdapt}`)
        }
        if (node.tagName === 'a' && colorAdapt) {
          attrs['data-dm-link'] = ''
          cssVars.push(`--dm-l: ${colorAdapt}`)
        }

        if (cssVars.length > 0) {
          node.properties!.style = style + '; ' + cssVars.join('; ')
          Object.assign(node.properties!, attrs)
        }
      }

      for (const child of node.children) {
        if (child.type === 'element') walk(child)
      }
    }
  }

  walk(tree)
  const stringifier = unified().use(rehypeStringify)
  const result = stringifier.stringify(tree)
  return `<style>${DM_CSS}</style>` + String(result) + `<style>${DM_RESET}</style>`
}

export {
  sanitizeHtmlRehype,
  linkifyPlainText,
  extractTextForTranslation,
  translateHtmlWithLayout,
  adaptHtmlForDarkMode,
}
