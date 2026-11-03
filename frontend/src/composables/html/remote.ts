import type { Root, Element } from 'hast'
import { hasRemoteRef } from './urls.ts'

// Remote-content blocking: count and strip `http(s)` references, and replace a
// blocked image with a labelled placeholder so the layout does not collapse.

export const BLOCKED_IMAGE_CSS = `
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

const URL_ATTRS = ['src', 'srcSet', 'srcset', 'poster', 'background', 'data'] as const
const SIZING_PROPERTIES = new Set([
  'width',
  'height',
  'max-width',
  'max-height',
  'min-width',
  'min-height',
  'aspect-ratio',
])

function parseStyleDeclarations(style: string): Map<string, string> {
  const declarations = new Map<string, string>()
  for (const declaration of style.split(';')) {
    const match = declaration.match(/^\s*([a-z-]+)\s*:\s*([^;]+)$/i)
    if (match) declarations.set(match[1].toLowerCase(), match[2].trim())
  }
  return declarations
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
  const originalStyle = typeof properties.style === 'string' ? properties.style : ''
  const declarations = parseStyleDeclarations(originalStyle)
  const widthValue = declarations.get('width') ?? ''
  const heightValue = declarations.get('height') ?? ''
  const hasWidth = declarations.has('width')
  const hasHeight = declarations.has('height')
  const responsiveWidth = /%\s*(?:!important\s*)?$/i.test(widthValue)
  const autoHeight = !hasHeight || /^auto\s*(?:!important\s*)?$/i.test(heightValue)
  const useAspectRatio = responsiveWidth && autoHeight && width > 0 && height > 0

  const classes = ['remote-blocked']
  if (maxSide > 0 && maxSide <= 2) {
    // A tracking pixel: keep its box (usually 1×1) so the layout is untouched.
    classes.push('remote-blocked--dot')
  } else if (!alt || (maxSide > 0 && maxSide <= 24)) {
    classes.push('remote-blocked--icon')
  }

  // Keep the sender's sizing declarations (width:100%, height:auto, etc.).
  // Replacing a responsive cover image with a fixed 1200px box was widening
  // the whole mail and producing a pointless horizontal scrollbar.
  const sizing = [...declarations]
    .filter(([property]) => SIZING_PROPERTIES.has(property))
    .map(([property, value]) => `${property}:${value}`)
  if (!hasWidth && width > 0) sizing.push(`width:${width}px`)
  if (!hasHeight && height > 0 && !useAspectRatio) sizing.push(`height:${height}px`)
  if (useAspectRatio) sizing.push(`aspect-ratio:${width} / ${height}`)
  const style = sizing.length > 0 ? sizing.join(';') : undefined

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

export function rehypeBlockRemoteContent(options: {
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
