import { unified } from 'unified'
import remarkParse from 'remark-parse'
import remarkGfm from 'remark-gfm'
import remarkBreaks from 'remark-breaks'
import remarkRehype from 'remark-rehype'
import rehypeStringify from 'rehype-stringify'
import {
  sanitizeEmailHtml,
  type EmailSanitizeOptions,
  type EmailSanitizeResult,
} from './sanitize.ts'

// Plain-text mail is treated as Markdown, then sent through the same sanitiser as
// HTML mail. Raw HTML in the message is not enabled in remark-rehype, and any
// links/images it does produce still pass through the mail schema and remote
// content blocker.

const markdown = unified()
  .use(remarkParse)
  .use(remarkGfm)
  // Plain-text mail usually hard-wraps lines. CommonMark would fold those into
  // spaces; breaks keeps the sender's visible line breaks.
  .use(remarkBreaks)
  .use(literalHtmlAsText)
  .use(remarkRehype)
  .use(rehypeStringify)

interface MarkdownNode {
  type: string
  value?: string
  children?: MarkdownNode[]
}

/**
 * A text/plain message may legitimately contain `<foo>` as text. Markdown treats
 * it as raw HTML and remark-rehype would drop it; turn HTML nodes back into text
 * so they are escaped and visible instead.
 */
function literalHtmlAsText() {
  return (tree: unknown) => {
    if (!tree || typeof tree !== 'object') return

    const walk = (node: MarkdownNode) => {
      if (node.type === 'html') {
        node.type = 'text'
        return
      }
      for (const child of node.children ?? []) walk(child)
    }
    walk(tree as MarkdownNode)
  }
}

const MARKDOWN_CSS = `
.markdown-body > :first-child { margin-top: 0; }
.markdown-body > :last-child { margin-bottom: 0; }
.markdown-body p { margin: 0 0 0.85em; }
.markdown-body h1,
.markdown-body h2,
.markdown-body h3,
.markdown-body h4,
.markdown-body h5,
.markdown-body h6 {
  margin: 1.25em 0 0.55em;
  line-height: 1.25;
  font-weight: 650;
}
.markdown-body h1 { font-size: 1.65em; }
.markdown-body h2 { font-size: 1.4em; }
.markdown-body h3 { font-size: 1.2em; }
.markdown-body ul,
.markdown-body ol { margin: 0.55em 0 1em; padding-left: 1.6em; }
.markdown-body li + li { margin-top: 0.25em; }
.markdown-body blockquote {
  margin: 1em 0;
  padding: 0.1em 0 0.1em 1em;
  border-left: 3px solid var(--n-border-color, #d8d8dc);
  opacity: 0.82;
}
.markdown-body code {
  padding: 0.15em 0.35em;
  border-radius: 4px;
  background: var(--n-action-color, #f5f6fa);
  font-family: var(--font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 0.9em;
}
.markdown-body pre {
  padding: 12px 14px;
  border: 1px solid var(--n-border-color, #e5e5e8);
  border-radius: 7px;
  background: var(--n-action-color, #f5f6fa);
}
.markdown-body pre code {
  padding: 0;
  border-radius: 0;
  background: transparent;
  font-size: inherit;
}
.markdown-body hr {
  margin: 1.35em 0;
  border: 0;
  border-top: 1px solid var(--n-border-color, #e5e5e8);
}
.markdown-body table { border-collapse: collapse; margin: 1em 0; }
.markdown-body th,
.markdown-body td {
  padding: 0.4em 0.65em;
  border: 1px solid var(--n-border-color, #d8d8dc);
  text-align: left;
}
.markdown-body th {
  background: var(--n-action-color, #f5f6fa);
  font-weight: 600;
}
.markdown-body a {
  color: var(--n-primary-color, #e11d48);
  text-decoration: underline;
}
`

export function markdownToHtml(text: string): string {
  if (!text.trim()) return ''
  return String(markdown.processSync(text))
}

/** Render a text/plain MIME body as Markdown, then apply the mail sanitiser. */
export function sanitizePlainTextEmail(
  text: string,
  options: EmailSanitizeOptions = {},
): EmailSanitizeResult {
  const html = markdownToHtml(text)
  if (!html) return { html: '', blockedRemote: 0, cssArt: 0 }
  const result = sanitizeEmailHtml(html, options)
  return {
    ...result,
    // Added after sanitising so the layout wrapper and its class always survive.
    html: `<div class="markdown-body">${result.html}</div><style>${MARKDOWN_CSS}</style>`,
  }
}
