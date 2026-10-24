/**
 * Regression tests for the proxy settings page's description of where an
 * account's mail actually goes (`views/settings/Proxy.vue` renders what these
 * functions decide).
 *
 * Run with: pnpm test   (Node ≥ 23 strips TypeScript types natively)
 *
 * The Go side is the authority (services/proxy.go, `proxyForAccount`); these
 * cases mirror its rules so the page cannot describe a route the dialer would not
 * take. The one that matters most is the first: an account with no Proxy block at
 * all — every account created before this option existed — must read as
 * "follows the global setting", not as "direct".
 */
import { proxyModeOf, resolveProxyRoute } from '../src/composables/proxy.ts'

let pass = 0
let fail = 0

function check(name: string, cond: boolean) {
  if (cond) {
    pass++
    console.log(`  ok  ${name}`)
  } else {
    fail++
    console.log(`  FAIL ${name}`)
  }
}

const off = { enabled: false, url: '' }
const on = (url = '127.0.0.1:7897') => ({ enabled: true, url })

// --- mode normalisation ---

check('a missing Proxy block reads as "follow"', proxyModeOf(undefined) === '')
check('a null Proxy reads as "follow"', proxyModeOf(null) === '')
check('an empty mode reads as "follow"', proxyModeOf({ Mode: '', URL: '' }) === '')
check('custom survives', proxyModeOf({ Mode: 'custom', URL: 'x:1' }) === 'custom')
check('off survives', proxyModeOf({ Mode: 'off', URL: '' }) === 'off')
check('an unknown mode falls back to "follow"', proxyModeOf({ Mode: 'socks', URL: '' }) === '')

// --- resolution ---

check(
  'follow + global off is a direct connection',
  resolveProxyRoute(undefined, off).kind === 'direct',
)

check(
  'follow + global on uses the global address',
  resolveProxyRoute(undefined, on()).kind === 'global',
)

check(
  'an enabled global proxy with no address is direct (nothing to dial)',
  resolveProxyRoute(undefined, { enabled: true, url: '   ' }).kind === 'direct',
)

check(
  'custom wins over the global setting',
  (() => {
    const route = resolveProxyRoute({ Mode: 'custom', URL: ' socks5://10.0.0.2:1080 ' }, on())
    return route.kind === 'custom' && route.address === 'socks5://10.0.0.2:1080'
  })(),
)

check(
  'custom with a blank address stays custom (the page has to be able to say so)',
  (() => {
    const route = resolveProxyRoute({ Mode: 'custom', URL: '  ' }, on())
    return route.kind === 'custom' && route.address === ''
  })(),
)

check(
  'off stays direct even with the global proxy on',
  resolveProxyRoute({ Mode: 'off', URL: '' }, on()).kind === 'direct',
)

check(
  'off ignores a stale custom address',
  resolveProxyRoute({ Mode: 'off', URL: '127.0.0.1:7897' }, on()).kind === 'direct',
)

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
