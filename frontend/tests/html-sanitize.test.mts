/**
 * Regression tests for the email body sanitizer (security-relevant).
 *
 * Run with: pnpm test   (Node ≥ 23 strips TypeScript types natively)
 *
 * Coverage: remote-content blocking + counting, the allowRemote escape hatch,
 * CSS sanitising (remote url(), @import, layout escapes) and markup attacks.
 */
import { sanitizeEmailHtml } from '../src/composables/html/sanitize.ts'

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

const block = (html: string) => sanitizeEmailHtml(html, { allowRemote: false })
const allow = (html: string) => sanitizeEmailHtml(html, { allowRemote: true })

console.log('remote images')
const remoteImg = block('<img src="http://tracker.example/p.png" alt="a">')
check('block: src removed', !/tracker\.example/.test(remoteImg.html))
check('block: counted', remoteImg.blockedRemote === 1)
check(
  'block: becomes a styled placeholder span',
  /<span[^>]*class="remote-blocked"/.test(remoteImg.html),
)
check('block: marker kept for styling/tests', /data-remote-blocked/.test(remoteImg.html))
check('block: alt text stays visible', />a<\/span>/.test(remoteImg.html))
check('block: no broken <img> left', !/<img/.test(remoteImg.html))
const allowedImg = allow('<img src="http://t.example/p.png">')
check(
  'allow: src kept and not counted',
  allowedImg.html.includes('t.example/p.png') && allowedImg.blockedRemote === 0,
)

console.log('placeholder sizing')
const tracker = block('<img src="http://t.example/px.gif" width="1" height="1">')
check(
  'block: 1x1 tracker keeps its own box (dot)',
  /remote-blocked--dot/.test(tracker.html) && /width:1px;height:1px/.test(tracker.html),
)
const smallIcon = block('<img src="http://t.example/i.png" width="16" height="16">')
check(
  'block: small image keeps its box + compact icon',
  /remote-blocked--icon/.test(smallIcon.html) && /width:16px;height:16px/.test(smallIcon.html),
)
const bigNoAlt = block('<img src="http://t.example/big.png" width="400" height="200">')
check(
  'block: large image without alt keeps its box + icon',
  /remote-blocked--icon/.test(bigNoAlt.html) && /width:400px;height:200px/.test(bigNoAlt.html),
)
const labelled = sanitizeEmailHtml('<img src="http://t.example/x.png">', {
  allowRemote: false,
  blockedImageLabel: 'Remote image blocked',
})
check(
  'block: placeholder tooltip is the caller label',
  labelled.html.includes('title="Remote image blocked"'),
)

console.log('url-bearing attributes')
check(
  'block: protocol-relative blocked',
  block('<img src="//t.example/p.png">').blockedRemote === 1,
)
const picture =
  '<picture><source srcset="http://t.example/a.png 1x"><img src="http://t.example/b.png"></picture>'
const pictureOut = block(picture)
check(
  'block: <picture><source srcset> blocked (camel-case srcSet)',
  pictureOut.blockedRemote === 2 && !/t\.example/.test(pictureOut.html),
)
check(
  'block: img srcset never survives',
  !/t\.example/.test(block('<img srcset="http://t.example/a.png 1x">').html),
)
check(
  'block: video/audio/iframe dropped by schema',
  !/t\.example/.test(
    block('<video src="http://t.example/v.mp4"></video><iframe src="http://t.example/f"></iframe>')
      .html,
  ),
)
check(
  'block: data: uri untouched',
  block('<img src="data:image/png;base64,AAA">').blockedRemote === 0,
)
check('block: relative url untouched', block('<img src="/local.png">').html.includes('/local.png'))
// Embedded (CID) images are delivered inside the message — the backend has already
// turned them into data: URIs, so they cause no request and are deliberately kept.
// Only references that leave the device are blocked.
check('block: cid: reference kept', block('<img src="cid:logo@example">').blockedRemote === 0)

console.log('inline style')
const cssRemote = block('<div style="background:url(http://t.example/x.png)">x</div>')
check(
  'block: remote url() -> none',
  cssRemote.html.includes('background:none') && cssRemote.blockedRemote === 1,
)
check(
  'block: comment-split url() still blocked',
  block('<div style="background:url(/*x*/http://t.example/x.png)">x</div>').blockedRemote === 1,
)
check(
  'block: data: url() kept',
  block('<div style="background:url(data:image/png;base64,AAA)">x</div>').blockedRemote === 0,
)
check(
  'allow: remote url() kept',
  allow('<div style="background:url(http://t.example/x.png)">x</div>').html.includes(
    't.example/x.png',
  ),
)

console.log('style blocks')
const styleBlock = block(
  '<style>@import url(http://evil/x.css); .a{background:url(http://evil/t.gif)}</style><p>x</p>',
)
check('block: @import + url() counted', styleBlock.blockedRemote === 2)
check('block: no remote reference left', !/evil/.test(styleBlock.html))
const styleAllowed = allow(
  '<style>@import url(http://evil/x.css); .a{background:url(http://evil/t.gif)}</style>',
)
check('allow: @import still stripped', !/@import/.test(styleAllowed.html))
check('allow: remote background kept', styleAllowed.html.includes('evil/t.gif'))
check('allow: no stale blocked count', styleAllowed.blockedRemote === 0)

console.log('css escapes')
check(
  'position:fixed neutralised',
  block('<div style="position:fixed;top:0">x</div>').html.includes('position: static'),
)
check(
  'behavior stripped',
  !/behavior/i.test(block('<div style="behavior:url(x.htc)">x</div>').html),
)
check(
  'expression() stripped',
  !/expression/i.test(block('<div style="width:expression(alert(1))">x</div>').html),
)

console.log('markup attacks')
check('script removed', !/<script/i.test(block('<script>alert(1)</script><p>ok</p>').html))
check('onerror removed', !/onerror/i.test(block('<img src="x" onerror="alert(1)">').html))
check('style break-out contained', !/alert/.test(block('</style><script>alert(1)</script>').html))

console.log('colour-scheme media queries')
// A mail ships both designs and WebKit resolves `prefers-color-scheme` against the OS,
// not our `data-theme` (WebKit bug 319500) — so with macOS in Dark and the app in Light,
// the OpenAI newsletter rendered `color:#fff` over its own white tables: invisible text.
// The condition is rewritten to a constant instead, so only the app theme decides.
const darkMqMail =
  '<style>@media (prefers-color-scheme: dark) { p { color: #ffffff } }</style><p>hi</p>'
const darkMqLightApp = sanitizeEmailHtml(darkMqMail, { allowRemote: false, darkMode: false })
const darkMqDarkApp = sanitizeEmailHtml(darkMqMail, { allowRemote: false, darkMode: true })
check(
  'light app: the OS dark design is switched off',
  darkMqLightApp.html.includes('(max-width: 0px)'),
)
check(
  'dark app: the OS dark design is switched on',
  darkMqDarkApp.html.includes('(min-width: 0px)'),
)
check(
  'the OS appearance can never decide the mail colours',
  !/prefers-color-scheme/.test(darkMqLightApp.html + darkMqDarkApp.html),
)
check(
  'light-only rules still apply in light mode',
  block('<style>@media (prefers-color-scheme: light) { p { color: #000 } }</style>').html.includes(
    '(min-width: 0px)',
  ),
)
check(
  'light-only rules are switched off in dark mode',
  sanitizeEmailHtml('<style>@media (prefers-color-scheme: light) { p { color: #000 } }</style>', {
    allowRemote: false,
    darkMode: true,
  }).html.includes('(max-width: 0px)'),
)
// The rest of a compound condition must survive: only the scheme half is replaced.
check(
  'compound conditions keep their other terms',
  sanitizeEmailHtml(
    '<style>@media screen and (prefers-color-scheme: dark) { p { color: #fff } }</style>',
    {
      allowRemote: false,
      darkMode: false,
    },
  ).html.includes('screen and (max-width: 0px)'),
)
check(
  'colour-scheme rules inside comments cannot break the rewrite',
  block(
    '<style>/* @media (prefers-color-scheme: dark) {} */ p { color: #000 }</style>',
  ).html.includes('color: #000'),
)
// Default (no darkMode given) is the readable light design.
check(
  'missing darkMode falls back to the light design',
  block(darkMqMail).html.includes('(max-width: 0px)'),
)

console.log('css-drawn images')
// A real spam body: one `box-shadow` declaration of 127 KB, one entry per pixel. It has no
// <img>, no url() and no attachment, so remote blocking cannot see it.
const pixelArt =
  '<div style="width: 1px; height: 1px; box-shadow: ' +
  Array.from({ length: 400 }, (_, i) => `${i}px 15px #123456`).join(',') +
  ';">x</div>'
const pixelOut = block(pixelArt).html
check('pixel-art shadow dropped', !/box-shadow/i.test(pixelOut))
check('rest of the rule kept', /width:\s*1px/.test(pixelOut))
check(
  'ordinary shadows kept',
  /box-shadow:\s*0 1px 2px/.test(block('<p style="box-shadow: 0 1px 2px #000">x</p>').html),
)
// The count drives the "drawn with CSS, likely spam" bar in the reader.
check('pixel art is counted for the notice', block(pixelArt).cssArt === 1)
check('normal mail reports no css art', block('<p style="color: red">hi</p>').cssArt === 0)

console.log('date/time mentions')
const mentionOptions = {
  enabled: true,
  locale: 'zh-CN',
  now: new Date(2026, 2, 1, 10, 0, 0),
  label: (at: Date) => `add reminder ${at.toISOString()}`,
}
const mentioned = sanitizeEmailHtml('<p>3月5日下午3点 评审</p>', {
  allowRemote: false,
  mentions: mentionOptions,
})
check('mention is wrapped', /<span class="date-mention"/.test(mentioned.html))
check('mention carries epoch seconds', /data-remind-at="\d{10}"/.test(mentioned.html))
check('mention keeps its text', mentioned.html.includes('3月5日下午3点'))
check('mention has a tooltip', mentioned.html.includes('add reminder'))
check('mention css injected', mentioned.html.includes('.date-mention'))
check(
  'no mention markup when disabled',
  !/<span class="date-mention"/.test(
    sanitizeEmailHtml('<p>3月5日下午3点</p>', { allowRemote: false }).html,
  ),
)
check(
  'dates inside links are left alone',
  !/<span class="date-mention"/.test(
    sanitizeEmailHtml('<a href="https://x.example/3月5日">3月5日</a>', {
      allowRemote: false,
      mentions: mentionOptions,
    }).html,
  ),
)
// Plain-text senders wrap the whole message in a <pre>; that must still be scanned.
const cmtStyle = sanitizeEmailHtml(
  '<!DOCTYPE html><html><head><meta charset="utf-8"></head><body><pre style="font-family: monospace">The review deadline has been extended to September 28, 2026, at 23:59 AoE (UTC-12).</pre></body></html>',
  {
    allowRemote: false,
    mentions: { ...mentionOptions, locale: 'en-US', now: new Date(2026, 8, 27, 15, 0) },
  },
)
check(
  'dates inside a <pre>-wrapped body are wrapped',
  /<span class="date-mention"/.test(cmtStyle.html),
)
check('pre-wrapped date keeps its text', cmtStyle.html.includes('September 28, 2026, at 23:59'))
check('chip carries the converted zone', cmtStyle.html.includes('data-remind-zone="AoE (UTC-12)"'))

// A zone named next to a bare date is part of the chip, but nothing was converted, so the
// dialog must not claim it was.
const bareZone = sanitizeEmailHtml('<p>deadline September 15, 2026 AOE</p>', {
  allowRemote: false,
  mentions: { ...mentionOptions, locale: 'en-US', now: new Date(2026, 8, 1, 9, 0) },
})
check('bare-date zone is highlighted', bareZone.html.includes('September 15, 2026 AOE'))
check('bare-date zone claims no conversion', !/data-remind-zone/.test(bareZone.html))
check(
  'dates inside code are still scanned (documented tradeoff)',
  /<span class="date-mention"/.test(
    sanitizeEmailHtml('<code>3月5日</code>', { allowRemote: false, mentions: mentionOptions }).html,
  ),
)
const hostileMention = sanitizeEmailHtml(
  '<span class="date-mention" data-remind-at="0" data-remind-text="x">点击</span>',
  { allowRemote: false },
)
check('email cannot fake a mention chip', !/data-remind-at/.test(hostileMention.html))
check(
  'email cannot fake the mention class',
  !/<span class="date-mention"/.test(hostileMention.html),
)

console.log('oversized content')
// Sanitiser wraps every <table> in a horizontally-scrollable <div>, so wide tables
// grow inside their own wrapper instead of the reading pane.
const wideTable = block(
  '<table width="800"><tr><td>wide</td></tr></table><p>after</p>',
)
check(
  'wide table is wrapped in email-table-scroll',
  /<div class="email-table-scroll"><table/.test(wideTable.html),
)
check(
  'table siblings stay outside the wrapper',
  wideTable.html.includes('<p>after</p>') &&
    wideTable.html.indexOf('<p>after</p>') > wideTable.html.indexOf('</div>'),
)
// Nested tables: each one gets its own wrapper, none are double-wrapped.
const nestedTable = block(
  '<table width="600"><tr><td><table width="400"><tr><td>nested</td></tr></table></td></tr></table>',
)
const wrapperCount = (nestedTable.html.match(/class="email-table-scroll"/g) ?? []).length
check(
  'outer and nested table each get exactly one wrapper',
  wrapperCount === 2 &&
    !/email-table-scroll"><div class="email-table-scroll"/.test(nestedTable.html),
)
check(
  'table-free bodies do not get an empty wrapper',
  !/<div class="email-table-scroll"/.test(block('<p>just text</p>').html),
)
// Content-fit CSS is always injected, regardless of options, so a stray oversized
// image declared with width="1200" or any inline-styled <img> shrinks to fit.
const fitCss = block('<p>x</p>').html
check(
  'content-fit img rule is injected',
  fitCss.includes('max-width: 100%') && fitCss.includes('height: auto'),
)
check(
  'content-fit table-scroll rule is injected',
  fitCss.includes('.email-table-scroll') && fitCss.includes('overflow-x: auto'),
)
const { linkifyPlainText } = await import('../src/composables/html/mentions.ts' as string)
check(
  'plain-text body also gets the fit rules',
  linkifyPlainText('hi').includes('email-table-scroll'),
)

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
