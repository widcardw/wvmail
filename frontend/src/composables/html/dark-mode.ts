import { unified } from 'unified'
import rehypeParse from 'rehype-parse'
import rehypeStringify from 'rehype-stringify'
import type { Root, Element } from 'hast'

type RGB = [number, number, number]

/** Named CSS colors supported by the email color adapter. */
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

function parseHex(hex: string): RGB | null {
  const value = hex.replace(/^#/, '')
  if (value.length === 3) {
    const [r, g, b] = value
    return [parseInt(r + r, 16), parseInt(g + g, 16), parseInt(b + b, 16)]
  }
  if (value.length === 6) {
    return [
      parseInt(value.slice(0, 2), 16),
      parseInt(value.slice(2, 4), 16),
      parseInt(value.slice(4, 6), 16),
    ]
  }
  return null
}

function parseColor(color: string): RGB | null {
  const value = color.trim().toLowerCase()
  if (!value) return null
  const named = NAMED_COLORS[value]
  if (named !== undefined) return named ? parseHex(named) : null
  if (value.startsWith('#')) return parseHex(value)

  const rgbMatch = value.match(/^rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/)
  if (rgbMatch) {
    return [parseInt(rgbMatch[1], 10), parseInt(rgbMatch[2], 10), parseInt(rgbMatch[3], 10)]
  }
  return null
}

function luminance(rgb: RGB): number {
  const [lr, lg, lb] = rgb.map((channel) => {
    const value = channel / 255
    return value <= 0.03928 ? value / 12.92 : Math.pow((value + 0.055) / 1.055, 2.4)
  })
  return 0.2126 * lr + 0.7152 * lg + 0.0722 * lb
}

function adaptColor(color: RGB, isBackground: boolean): string {
  const value = luminance(color)
  if (isBackground) {
    if (value > 0.6) return `rgb(${255 - color[0]},${255 - color[1]},${255 - color[2]})`
    if (value > 0.35) return 'rgb(45,45,45)'
    return ''
  }
  return value < 0.4 ? 'rgb(210,210,210)' : ''
}

const DM_CSS = `
:host { color-scheme: dark; }
[data-dm-color] { color: var(--dm-c) !important; }
[data-dm-preserve-color] { color: var(--dm-original-color) !important; }
[data-dm-bg] { background-color: var(--dm-b) !important; }
[data-dm-link] { color: var(--dm-l) !important; }
[data-dm-border-all] { border-color: var(--n-border-color) !important; }
[data-dm-border-top] { border-top-color: var(--n-border-color) !important; }
[data-dm-border-right] { border-right-color: var(--n-border-color) !important; }
[data-dm-border-bottom] { border-bottom-color: var(--n-border-color) !important; }
[data-dm-border-left] { border-left-color: var(--n-border-color) !important; }
[data-dm-code-border] { border-color: var(--n-border-color) !important; }
`

const DM_RESET = `
* { color: rgb(210,210,210) !important; }
:where(*) { border-color: var(--n-border-color); }
.email-auto-link { color: var(--n-primary-color, #66b0ff) !important; text-decoration: underline; }
* { background: transparent !important; }
[data-dm-bg] { background-color: var(--dm-b) !important; }
img { background: none !important; }
`

function parseBackgroundColor(value: string): RGB | null {
  const token = value.trim().match(/^(#[0-9a-f]{3,8}\b|(?:rgba?|hsla?)\s*\([^)]*\)|[a-z]+)\b/i)
  return token ? parseColor(token[1]) : null
}

function findBorderColorToken(value: string): string | null {
  const namedColors = Object.keys(NAMED_COLORS).join('|')
  const token = value.match(
    new RegExp(`#[0-9a-f]{3,8}\\b|(?:rgba?|hsla?)\\s*\\([^)]*\\)|\\b(?:${namedColors})\\b`, 'i'),
  )
  return token?.[0] ?? null
}

function isLightNeutralColor(color: RGB): boolean {
  const range = Math.max(...color) - Math.min(...color)
  return range <= 32 && luminance(color) > 0.55
}

function rewriteImportantBackgrounds(style: string): string {
  const declaration = /(\bbackground(?:-color)?\s*:\s*)([^;]*?)(\s*!important)(\s*;|$)/gi
  return style.replace(
    declaration,
    (full, prefix: string, value: string, _important: string, end: string) => {
      const color = parseBackgroundColor(value)
      if (!color) return full
      const replacement =
        luminance(color) > 0.6 ? 'transparent' : adaptColor(color, true) || 'transparent'
      return `${prefix}${replacement} !important${end}`
    },
  )
}

function rewriteImportantNeutralBorders(style: string): string {
  const declaration =
    /(\bborder(?:-(?:top|right|bottom|left))?(?:-color|-style|-width)?\s*:\s*)([^;]*?)(\s*!important)(\s*;|$)/gi

  return style.replace(
    declaration,
    (full: string, prefix: string, value: string, important: string, end: string) => {
      const token = findBorderColorToken(value)
      const color = token ? parseColor(token) : null
      if (!token || !color || !isLightNeutralColor(color)) return full

      const replacement = /\bborder(?:-(?:top|right|bottom|left))?-color\s*:/i.test(prefix)
        ? 'var(--n-border-color)'
        : value.replace(token, 'var(--n-border-color)')
      return `${prefix}${replacement}${important}${end}`
    },
  )
}

function appendInlineStyle(style: string, declaration: string): string {
  if (!declaration) return style
  const trimmed = style.trim()
  return trimmed ? `${trimmed.replace(/;?$/, ';')} ${declaration}` : declaration
}

function markDefaultBorderAttributes(style: string, attrs: Record<string, string>): void {
  const sides = ['top', 'right', 'bottom', 'left'] as const
  const defaultSides = new Set<(typeof sides)[number]>()
  const explicitSides = new Set<(typeof sides)[number]>()
  const colorPattern =
    /#[0-9a-f]{3,8}\b|\b(?:rgba?|hsla?)\s*\(|\b(?:transparent|currentcolor)\b|var\(/i

  for (const declaration of style.split(';')) {
    const match = declaration.match(
      /^\s*(border(?:-(top|right|bottom|left))?(?:-color|-style|-width)?)\s*:\s*([\s\S]+)$/i,
    )
    if (!match) continue

    const value = match[3]
    const side = match[2]?.toLowerCase() as (typeof sides)[number] | undefined
    const affectedSides = side ? [side] : sides
    const token = findBorderColorToken(value)
    const borderColor = token ? parseColor(token) : null

    for (const affectedSide of affectedSides) {
      if (borderColor && isLightNeutralColor(borderColor)) {
        if (!explicitSides.has(affectedSide)) defaultSides.add(affectedSide)
      } else if (colorPattern.test(value)) {
        explicitSides.add(affectedSide)
        defaultSides.delete(affectedSide)
      } else if (!explicitSides.has(affectedSide)) {
        defaultSides.add(affectedSide)
      }
    }
  }

  if (defaultSides.size === sides.length) {
    attrs['data-dm-border-all'] = ''
    return
  }
  for (const side of defaultSides) attrs[`data-dm-border-${side}`] = ''
}

function adaptHtmlForDarkMode(html: string): string {
  const parser = unified().use(rehypeParse, { fragment: true })
  const tree = parser.parse(html)

  function walk(node: Root | Element): void {
    if (!('children' in node)) return
    if (node.type === 'element') {
      const style = (node.properties?.style as string) ?? ''
      let adaptedStyle = style
      const attrs: Record<string, string> = {}
      const cssVars: string[] = []

      if (node.tagName === 'hr' && !style) attrs['data-dm-border-all'] = ''

      if (node.tagName === 'code' || node.tagName === 'pre') {
        attrs['data-dm-code-border'] = ''
        adaptedStyle = appendInlineStyle(
          adaptedStyle,
          'border-color: var(--n-border-color) !important',
        )
      }

      if (style) {
        adaptedStyle = rewriteImportantBackgrounds(adaptedStyle)
        adaptedStyle = rewriteImportantNeutralBorders(adaptedStyle)
        let colorAdapt = ''
        let bgAdapt = ''

        const colorMatch = adaptedStyle.match(/(?:^|;)\s*color\s*:\s*([^;]+)/i)
        if (colorMatch) {
          const originalColor = colorMatch[1].trim().replace(/\s*!important\s*$/i, '')
          const parsed = parseColor(originalColor)
          if (parsed) {
            colorAdapt = adaptColor(parsed, false)
            if (!colorAdapt) {
              attrs['data-dm-preserve-color'] = ''
              cssVars.push(`--dm-original-color: ${originalColor}`)
            }
          }
        }

        const bgMatch = adaptedStyle.match(/\bbackground-color\s*:\s*([^;]+)/i)
        if (bgMatch) {
          const parsed = parseColor(bgMatch[1])
          if (parsed) bgAdapt = adaptColor(parsed, true)
        }

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

        markDefaultBorderAttributes(adaptedStyle, attrs)
        if (adaptedStyle !== style || cssVars.length > 0 || Object.keys(attrs).length > 0) {
          node.properties!.style = appendInlineStyle(adaptedStyle, cssVars.join('; '))
          Object.assign(node.properties!, attrs)
        }
      } else if (Object.keys(attrs).length > 0) {
        node.properties!.style = adaptedStyle
        Object.assign(node.properties!, attrs)
      }
    }

    for (const child of node.children) {
      if (child.type === 'element') walk(child)
    }
  }

  walk(tree)
  const stringifier = unified().use(rehypeStringify)
  const result = stringifier.stringify(tree)
  return `<style>${DM_CSS}</style>` + String(result) + `<style>${DM_RESET}</style>`
}

export { adaptHtmlForDarkMode }
