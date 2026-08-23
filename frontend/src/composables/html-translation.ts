import { unified } from 'unified'
import rehypeParse from 'rehype-parse'
import rehypeStringify from 'rehype-stringify'
import { toText } from 'hast-util-to-text'
import type { Root, Element, Text } from 'hast'

const TRANSLATION_SKIP_TAGS = new Set(['a', 'style', 'script', 'pre', 'code', 'noscript'])
const SEPARATOR = '\n————\n'

function extractTextForTranslation(html: string): string {
  const parser = unified().use(rehypeParse, { fragment: true })
  return toText(parser.parse(html))
}

function collectAndInject(tree: Root): {
  texts: string[]
  inject: (translated: string[]) => boolean
} {
  const texts: string[] = []
  const textNodes: { node: Text; parent: Element; index: number }[] = []

  function walk(node: Root | Element): void {
    if (!('children' in node)) return
    if (node.type === 'element' && TRANSLATION_SKIP_TAGS.has(node.tagName)) return

    const children = node.children
    for (let i = children.length - 1; i >= 0; i--) {
      const child = children[i]
      if (child.type === 'text') {
        const value = child.value.trim()
        if (value) {
          texts.push(child.value)
          textNodes.push({ node: child, parent: node as Element, index: i })
        }
      } else if (child.type === 'element') {
        walk(child)
      }
    }
  }

  walk(tree)
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

/** Translate HTML text nodes while preserving the original element layout. */
async function translateHtmlWithLayout(
  html: string,
  translateFn: (text: string) => Promise<string>,
): Promise<string | null> {
  const parser = unified().use(rehypeParse, { fragment: true })
  const tree = parser.parse(html)
  const { texts, inject } = collectAndInject(tree)

  if (texts.length === 0) return null

  const translated = await translateFn(texts.join(SEPARATOR))
  const parts = translated.split(SEPARATOR)
  if (parts.length !== texts.length) return null
  if (!inject(parts.map((part) => part.trim()))) return null

  const stringifier = unified().use(rehypeStringify)
  return String(stringifier.stringify(tree))
}

export { extractTextForTranslation, translateHtmlWithLayout }
