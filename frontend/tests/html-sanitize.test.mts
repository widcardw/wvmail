/**
 * Regression tests for the email body sanitizer (security-relevant).
 *
 * Run with: pnpm test   (Node ≥ 23 strips TypeScript types natively)
 *
 * Coverage: remote-content blocking + counting, the allowRemote escape hatch,
 * CSS sanitising (remote url(), @import, layout escapes) and markup attacks.
 */
import { sanitizeEmailHtml } from '../src/composables/html-sanitize.ts'

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

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
