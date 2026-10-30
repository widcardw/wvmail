/**
 * Plain-text email → Markdown → safe HTML regression tests.
 *
 * Run with: pnpm test
 */
import { sanitizePlainTextEmail } from '../src/composables/html/markdown.ts'

let pass = 0
let fail = 0

function check(name: string, cond: boolean) {
  if (cond) {
    pass++
    console.log(`  PASS  ${name}`)
  } else {
    fail++
    console.error(`  FAIL  ${name}`)
  }
}

const render = (text: string) => sanitizePlainTextEmail(text, { allowRemote: false })

console.log('markdown rendering')
const formatted = render(
  '# Release\n\n**ready** for [review](https://example.com/review)\n\n- one\n- two',
)
check('heading converted', /<h1[^>]*>Release<\/h1>/.test(formatted.html))
check('strong emphasis converted', /<strong>ready<\/strong>/.test(formatted.html))
check(
  'markdown link converted',
  /<a[^>]*href="https:\/\/example\.com\/review"[^>]*>review<\/a>/.test(formatted.html),
)
check('list converted', /<ul>[\s\S]*<li>one<\/li>[\s\S]*<li>two<\/li>/.test(formatted.html))
check('markdown wrapper survives sanitising', formatted.html.includes('class="markdown-body"'))

const wrapped = render('first line\nsecond line')
check('plain-text line breaks are preserved', wrapped.html.includes('first line<br>second line'))

const gfm = render(
  '| Name | State |\n| --- | --- |\n| sync | done |\n\n~~old~~ https://example.com',
)
check('GFM table converted', /<table>[\s\S]*<th>Name<\/th>/.test(gfm.html))
check('GFM strikethrough converted', /<del>old<\/del>/.test(gfm.html))
check(
  'GFM autolink converted',
  /<a[^>]*href="https:\/\/example\.com"[^>]*>https:\/\/example\.com<\/a>/.test(gfm.html),
)

const code = render('Use `pnpm test`.\n\n```ts\nconst ok = true\n```')
check('inline code converted', /<code>pnpm test<\/code>/.test(code.html))
check('fenced code converted', /<pre><code class="language-ts">/.test(code.html))

console.log('safety')
const rawHtml = render('<script>alert(1)</script><img src=x onerror=alert(2)>hello')
check('raw HTML is not emitted', !/<script|onerror/i.test(rawHtml.html))
check('raw HTML text remains visible', rawHtml.html.includes('hello'))

const unsafeLink = render('[click](javascript:alert(1))')
check('javascript link removed', !/javascript:/i.test(unsafeLink.html))

const remoteImage = render('![tracker](https://tracker.example/pixel.png)')
check('markdown image is blocked', !/tracker\.example/.test(remoteImage.html))
check('markdown image counts as remote content', remoteImage.blockedRemote === 1)

const mentioned = sanitizePlainTextEmail('Review **tomorrow** at 3pm', {
  allowRemote: false,
  mentions: {
    enabled: true,
    locale: 'en-US',
    now: new Date(2026, 9, 30, 9, 0, 0),
    label: () => 'add reminder',
  },
})
check(
  'date mentions still run after Markdown conversion',
  /<span class="date-mention"/.test(mentioned.html),
)
check('Markdown emphasis and mentions coexist', /<strong>.*date-mention/.test(mentioned.html))

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
