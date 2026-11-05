import { unified } from 'unified'
import rehypeParse from 'rehype-parse'
import rehypeSanitize from 'rehype-sanitize'
import rehypeStringify from 'rehype-stringify'
import { rehypeSanitizeEmailCss } from './css.ts'
import {
  MENTION_CSS,
  type MentionRenderOptions,
  THEME_LINK_CSS,
  linkifyPlainText,
  rehypeAutoLink,
  rehypeDateMentions,
} from './mentions.ts'
import { BLOCKED_IMAGE_CSS, rehypeBlockRemoteContent, type RemoteURLRewriter } from './remote.ts'
import { emailSchema } from './schema.ts'
import { rehypeWrapTables } from './wrap-tables.ts'
import { CONTENT_FIT_CSS } from './content-fit.ts'

// The pipeline: parse → sanitize (schema.ts) → CSS (css.ts) → wrap tables
// (wrap-tables.ts) → remote content (remote.ts) → mentions (mentions.ts) →
// stringify, with a per-mode sanitizer cached because the pipeline is
// expensive to rebuild on every message.

export interface EmailSanitizeOptions {
  /** Load remote images/fonts/imported CSS. Default false — remote content is blocked. */
  allowRemote?: boolean
  /** Tooltip for blocked-image placeholders (translated by the caller). */
  blockedImageLabel?: string
  /**
   * Base URL of the image proxy. When remote content is allowed, every remote
   * image and CSS `url()` is rewritten to `<base>?url=<encoded target>` so the
   * fetch happens off-device. Omit to leave URLs untouched (tests / direct mode).
   */
  imageProxyURL?: string
  /** Highlight detected date/time mentions (off unless `enabled`). */
  mentions?: MentionRenderOptions
  /**
   * The theme the body is about to be rendered in. Decides which of the mail's own
   * `prefers-color-scheme` rules survive — the OS appearance deliberately does not.
   * Defaults to light, so a caller that forgets gets the readable design.
   */
  darkMode?: boolean
}

export interface EmailSanitizeResult {
  html: string
  /** Remote resources that were removed (image sources plus CSS references). */
  blockedRemote: number
  /**
   * Oversized style declarations that were dropped. A message whose content is drawn by
   * CSS (one `box-shadow` entry per pixel) has nothing else to show, so the UI warns that
   * it is likely spam rather than silently rendering an empty body.
   */
  cssArt: number
}

// Two reusable pipelines (blocking / allowing remote content); building them is
// the expensive part, so they are cached per mode.
const sanitizers = new Map<boolean, ReturnType<typeof createSanitizer>>()

function createSanitizer(allowRemote: boolean) {
  let blockedRemote = 0
  let cssArt = 0
  // Whether the body will be shown on our dark canvas; a mail ships both colour
  // schemes and we pick, so this is read per call like the label below.
  let appDark = false
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
  let imageProxy: RemoteURLRewriter | undefined
  // Same for the date/time pass: toggle, locale and clock all come per call.
  let mentionOptions: MentionRenderOptions = {
    enabled: false,
    locale: 'en-US',
    now: new Date(0),
  }
  const processor = unified()
    .use(rehypeParse, { fragment: true })
    .use(rehypeAutoLink)
    .use(rehypeSanitize, emailSchema)
    // These run after sanitising, so they only rewrite/remove what it kept.
    .use(rehypeSanitizeEmailCss, {
      allowRemote,
      appDark: () => appDark,
      rewriteRemoteURL: () => imageProxy,
      onBlocked,
      onCssArt: (n: number) => {
        cssArt += n
      },
    })
    .use(rehypeWrapTables)
    .use(rehypeBlockRemoteContent, {
      allowRemote,
      onBlocked,
      imageLabel: () => imageLabel,
      proxyURL: () => imageProxy,
    })
    .use(rehypeDateMentions, () => mentionOptions)
    .use(rehypeStringify)
  return {
    processor,
    setImageLabel: (label: string) => {
      imageLabel = label
    },
    setImageProxy: (rewrite: RemoteURLRewriter | undefined) => {
      imageProxy = rewrite
    },
    setAppDark: (dark: boolean) => {
      appDark = dark
    },
    setMentionOptions: (options: MentionRenderOptions) => {
      mentionOptions = options
    },
    takeBlocked: () => {
      const n = blockedRemote
      blockedRemote = 0
      return n
    },
    takeCssArt: () => {
      const n = cssArt
      cssArt = 0
      return n
    },
  }
}

/**
 * Build the rewriter that turns an off-device URL into one the proxy fetches.
 * The proxy URL is encoded as a query value, so a URL containing `&` or `#`
 * cannot change the proxy's own parameters.
 */
function buildImageProxyRewriter(base: string): RemoteURLRewriter | undefined {
  const normalized = base.trim()
  if (!normalized) return undefined
  return (target: string) => {
    // A second sanitize pass (AI output, cached body) must not proxy a URL that
    // already points at this proxy. Compare parsed origins and paths instead of
    // a string prefix, so a lookalike URL that merely starts with the base text
    // is still proxied.
    if (isAlreadyProxied(target, normalized)) return target
    const separator = !normalized.includes('?')
      ? '?'
      : normalized.endsWith('?') || normalized.endsWith('&')
        ? ''
        : '&'
    return `${normalized}${separator}url=${encodeURIComponent(target)}`
  }
}

function isAlreadyProxied(target: string, base: string): boolean {
  try {
    const placeholder = 'http://image-proxy.invalid'
    const targetURL = new URL(target, placeholder)
    const baseURL = new URL(base, placeholder)
    if (targetURL.origin !== baseURL.origin || targetURL.pathname !== baseURL.pathname) {
      return false
    }
    if (!targetURL.searchParams.has('url')) return false
    for (const [key, value] of baseURL.searchParams) {
      if (targetURL.searchParams.get(key) !== value) return false
    }
    return true
  } catch {
    return false
  }
}

/**
 * Sanitize an email body: strips dangerous markup, neutralises unsafe CSS and
 * (unless `allowRemote`) removes remote images/styles and reports how many were
 * blocked, so the UI can offer to load them.
 *
 * The result depends on `darkMode`: the mail's own colour-scheme rules are rewritten
 * to match the theme we are rendering in (see `alignColorSchemeMediaQueries`), so the
 * caller must re-run this when the theme changes.
 */

function sanitizeEmailHtml(html: string, options: EmailSanitizeOptions = {}): EmailSanitizeResult {
  const allowRemote = options.allowRemote === true
  let entry = sanitizers.get(allowRemote)
  if (!entry) {
    entry = createSanitizer(allowRemote)
    sanitizers.set(allowRemote, entry)
  }
  entry.setImageLabel(options.blockedImageLabel ?? '')
  entry.setImageProxy(
    allowRemote && options.imageProxyURL
      ? buildImageProxyRewriter(options.imageProxyURL)
      : undefined,
  )
  entry.setAppDark(options.darkMode === true)
  const mentions = options.mentions
  entry.setMentionOptions(mentions ?? { enabled: false, locale: 'en-US', now: new Date(0) })
  const result = entry.processor.processSync(html)
  const blockedRemote = entry.takeBlocked()
  return {
    html:
      String(result) +
      `<style>${THEME_LINK_CSS}${BLOCKED_IMAGE_CSS}${CONTENT_FIT_CSS}${mentions?.enabled ? MENTION_CSS : ''}</style>`,
    blockedRemote,
    cssArt: entry.takeCssArt(),
  }
}

/** Sanitize HTML while preserving email layout and safe inline styles. */
function sanitizeHtmlRehype(html: string, options: EmailSanitizeOptions = {}): string {
  return sanitizeEmailHtml(html, options).html
}

export { sanitizeHtmlRehype, sanitizeEmailHtml, linkifyPlainText }
