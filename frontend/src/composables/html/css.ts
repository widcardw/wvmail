import type { Root, Element } from 'hast'
import { REMOTE_URL_RE } from './urls.ts'

// CSS sanitising: strip `@import`/`expression()`/`behavior`, block or rewrite remote
// `url()`, pin `position: fixed|sticky`, drop absurdly long declarations, and take
// the `prefers-color-scheme` decision away from the OS (see html-sanitize docs).

const MAX_DECLARATION_LENGTH = 2048

/** Drop `<property>: <absurdly long value>` declarations, keeping the rest of the rule. */
function dropOversizedDeclarations(css: string): { css: string; dropped: number } {
  if (css.length <= MAX_DECLARATION_LENGTH) return { css, dropped: 0 }
  let dropped = 0
  const next = css.replace(
    /([a-zA-Z-]+)\s*:\s*([^;{}]*)/g,
    (whole, _prop: string, value: string) => {
      if (value.length <= MAX_DECLARATION_LENGTH) return whole
      dropped++
      return ''
    },
  )
  return { css: next, dropped }
}

/**
 * Make the mail's own `prefers-color-scheme` rules follow *our* theme.
 *
 * The engine resolves that media query against the **OS appearance**, not against the
 * `color-scheme` we set for `data-theme` (WebKit tracks the CSSWG resolution that would
 * change this as bug 319500, still open). With macOS in Dark and the app in Light, every
 * `@media (prefers-color-scheme: dark)` block in a mail fired anyway — the OpenAI
 * newsletter then painted `color:#fff !important` + `-webkit-text-fill-color:#fff` over
 * its own inline white tables, i.e. an invisible body in light mode. Its matching dark
 * *backgrounds* never applied either, because they target `body` / `.wrapperMod`, and
 * the body is parsed as a fragment.
 *
 * Both conditions are therefore rewritten to constants the OS cannot influence:
 * `(min-width: 0px)` is always true, `(max-width: 0px)` always false — media queries
 * resolve against the viewport, and no window is zero pixels wide.
 */

const COLOR_SCHEME_MQ_RE = /\(\s*prefers-color-scheme\s*:\s*(dark|light)\s*\)/gi

export function alignColorSchemeMediaQueries(css: string, appDark: boolean): string {
  return css.replace(COLOR_SCHEME_MQ_RE, (_whole, scheme: string) =>
    (scheme.toLowerCase() === 'dark') === appDark ? '(min-width: 0px)' : '(max-width: 0px)',
  )
}

function sanitizeCss(
  css: string,
  allowRemote: boolean,
  appDark: boolean,
): { css: string; blocked: number; cssArt: number } {
  let blocked = 0
  let cssArt = 0
  // Comments are dropped first: they can split the tokens matched below.
  let out = css.replace(/\/\*[\s\S]*?\*\//g, ' ')

  // The sender's colour scheme must agree with the one we are rendering in.
  out = alignColorSchemeMediaQueries(out, appDark)

  // @import pulls an arbitrary external stylesheet — never allowed.
  out = out.replace(/@import[^;]*;?/gi, () => {
    blocked++
    return ''
  })

  if (!allowRemote) {
    out = out.replace(
      /url\(\s*(['"]?)([^'")]*)\1\s*\)/gi,
      (whole: string, _q: string, target: string) => {
        if (REMOTE_URL_RE.test(String(target).trim())) {
          blocked++
          return 'none'
        }
        return whole
      },
    )
  }

  // Legacy code-execution / binding vectors, and layout that could overlay the app.
  out = out.replace(/(?:-moz-binding|behavior)\s*:[^;}]*;?/gi, '')
  out = out.replace(/expression\s*\([^)]*\)/gi, '')
  out = out.replace(/position\s*:\s*(?:fixed|sticky)/gi, 'position: static')

  // A frozen `white-space: pre` / `nowrap` keeps a line on one row and pushes
  // past the reading-pane edge. Rewrite to a wrap-friendly value so it cannot
  // blow out the layout. `pre-wrap` / `pre-line` (which already wrap) are left
  // untouched — the `(?![-\w])` lookahead skips them. This is targeted at the
  // declaration itself, so normal body layout (default `normal`, or any table /
  // div the sender did not freeze) is never touched — unlike a global
  // `white-space: pre-wrap` rule, which would preserve every structural newline
  // in the source and inflate the whole message with blank rows.
  out = out.replace(/(white-space\s*:\s*)pre(?![-\w])/gi, '$1pre-wrap')
  out = out.replace(/(white-space\s*:\s*)nowrap(?![-\w])/gi, '$1normal')

  // CSS-drawn "images" (a box-shadow entry per pixel) are spam payload, not styling.
  const sized = dropOversizedDeclarations(out)
  cssArt = sized.dropped
  out = sized.css

  return { css: out, blocked, cssArt }
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
export function rehypeSanitizeEmailCss(options: {
  allowRemote: boolean
  appDark: () => boolean
  onBlocked: (n: number) => void
  onCssArt: (n: number) => void
}) {
  return (tree: Root) => {
    walkElements(tree, (el) => {
      if (el.tagName === 'style') {
        for (const child of el.children) {
          if (child.type !== 'text') continue
          const result = sanitizeCss(child.value, options.allowRemote, options.appDark())
          child.value = result.css
          options.onBlocked(result.blocked)
          options.onCssArt(result.cssArt)
        }
        return
      }
      const style = el.properties?.['style']
      if (typeof style !== 'string') return
      const result = sanitizeCss(style, options.allowRemote, options.appDark())
      el.properties!['style'] = result.css
      options.onBlocked(result.blocked)
      options.onCssArt(result.cssArt)
    })
  }
}
