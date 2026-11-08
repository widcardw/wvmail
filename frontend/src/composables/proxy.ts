import type { AccountProxy } from '#/wvmail/services'
import type { TranslationKey } from '~/i18n'

/**
 * How one account reaches its mail server, as far as the UI is concerned.
 *
 * The backend owns the actual rule (services/proxy.go); this mirrors it so the
 * settings page can *describe* the outcome without asking Go on every render.
 * The two must stay in step — `resolveProxyRoute` answers the same question
 * `proxyForAccount` does, for display only.
 */
export type ProxyMode = '' | 'custom' | 'off'

export type ProxyRoute =
  | { kind: 'direct' }
  | { kind: 'global'; address: string }
  | { kind: 'custom'; address: string }

/** The account's mode, normalised: anything unknown reads as "follow global". */
export function proxyModeOf(proxy: AccountProxy | null | undefined): ProxyMode {
  const mode = proxy?.Mode ?? ''
  return mode === 'custom' || mode === 'off' ? mode : ''
}

/**
 * Where this account's connections go.
 *
 * `off` beats the global switch (that is the point of it: a mailbox on the local
 * network must not be sent through a remote tunnel), `custom` beats it too, and
 * everything else follows the global proxy — including an enabled global proxy
 * with no address yet, which resolves to "direct" because that is what would
 * happen anyway (Go rejects it at dial time).
 */
export function resolveProxyRoute(
  proxy: AccountProxy | null | undefined,
  global: { enabled: boolean; url: string },
): ProxyRoute {
  const mode = proxyModeOf(proxy)
  if (mode === 'off') return { kind: 'direct' }
  if (mode === 'custom') return { kind: 'custom', address: (proxy?.URL ?? '').trim() }

  const address = global.url.trim()
  return global.enabled && address ? { kind: 'global', address } : { kind: 'direct' }
}

/** The three mode labels, shared by the proxy page and the account form. */
export function proxyModeLabels(t: Record<TranslationKey, string>): Array<{
  label: string
  value: ProxyMode
}> {
  return [
    { label: t['settings.proxy.mode_follow'], value: '' },
    { label: t['settings.proxy.mode_custom'], value: 'custom' },
    { label: t['settings.proxy.mode_off'], value: 'off' },
  ]
}
