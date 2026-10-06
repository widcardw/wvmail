import { unified } from 'unified'
import rehypeParse from 'rehype-parse'
import rehypeStringify from 'rehype-stringify'
import { toText } from 'hast-util-to-text'
import type { Root, Element, Text } from 'hast'
import { DEFAULT_LLM_OUTPUT_TOKENS } from '~/types/config'

const TRANSLATION_SKIP_TAGS = new Set(['a', 'style', 'script', 'pre', 'code', 'noscript'])
const SEPARATOR = '\n————\n'
const DEFAULT_TRANSLATION_CHARS = 6000
const PROMPT_OVERHEAD_TOKENS = 256
const CHARS_PER_TOKEN = 3.5

type TranslationModel =
  | {
      ContextWindowTokens?: number
      MaxOutputTokens?: number
    }
  | null
  | undefined

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

function getTranslationMaxChars(config: TranslationModel): number {
  const contextTokens = config?.ContextWindowTokens || 0
  if (!contextTokens) return DEFAULT_TRANSLATION_CHARS

  const outputTokens = config?.MaxOutputTokens || DEFAULT_LLM_OUTPUT_TOKENS
  const inputTokens = Math.max(contextTokens - outputTokens - PROMPT_OVERHEAD_TOKENS, 1)
  return Math.floor(inputTokens * CHARS_PER_TOKEN)
}

function createTranslationBatches(texts: string[], maxChars: number): string[][] {
  const batches: string[][] = []
  let batch: string[] = []
  let length = 0

  for (const text of texts) {
    const nextLength = length + (batch.length > 0 ? SEPARATOR.length : 0) + text.length
    if (batch.length > 0 && nextLength > maxChars) {
      batches.push(batch)
      batch = []
      length = 0
    }
    batch.push(text)
    length += (batch.length > 1 ? SEPARATOR.length : 0) + text.length
  }

  if (batch.length > 0) batches.push(batch)
  return batches
}

async function translateTexts(
  texts: string[],
  translateFn: (text: string) => Promise<string>,
  maxChars: number,
): Promise<string[]> {
  const translated: string[] = []
  for (const batch of createTranslationBatches(texts, maxChars)) {
    const result = await translateFn(batch.join(SEPARATOR))
    const parts = result.split(SEPARATOR)
    if (parts.length !== batch.length) return []
    translated.push(...parts.map((part) => part.trim()))
  }
  return translated
}

/** Translate HTML text nodes while preserving the original element layout. */
async function translateHtmlWithLayout(
  html: string,
  translateFn: (text: string) => Promise<string>,
  maxChars = DEFAULT_TRANSLATION_CHARS,
): Promise<string | null> {
  const parser = unified().use(rehypeParse, { fragment: true })
  const tree = parser.parse(html)
  const { texts, inject } = collectAndInject(tree)

  if (texts.length === 0) return null

  const translated = await translateTexts(texts, translateFn, maxChars)
  if (translated.length !== texts.length || !inject(translated)) return null

  const stringifier = unified().use(rehypeStringify)
  return String(stringifier.stringify(tree))
}

async function translatePlainText(
  text: string,
  translateFn: (text: string) => Promise<string>,
  maxChars = DEFAULT_TRANSLATION_CHARS,
): Promise<string> {
  const parts = text.split(/(\n\s*\n)/)
  const paragraphIndexes = parts.reduce<number[]>((indexes, part, index) => {
    if (part.trim()) indexes.push(index)
    return indexes
  }, [])
  const translated = await translateTexts(
    paragraphIndexes.map((index) => parts[index]),
    translateFn,
    maxChars,
  )
  if (translated.length !== paragraphIndexes.length) return ''

  for (let i = 0; i < paragraphIndexes.length; i++) {
    parts[paragraphIndexes[i]] = translated[i]
  }
  return parts.join('')
}

export {
  extractTextForTranslation,
  getTranslationMaxChars,
  translateHtmlWithLayout,
  translatePlainText,
}
