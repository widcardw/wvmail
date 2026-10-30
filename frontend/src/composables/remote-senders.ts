/**
 * Allowed remote-content sender matching.
 *
 * Supported forms:
 *  - `user@example.com` — exact address
 *  - `@example.com` — every address on that domain
 *  - `news@*.example.com` — one wildcard label per dot-separated label
 *  - `user@example.*` — a wildcard in the domain suffix
 *
 * A wildcard never crosses a domain label or the local/domain boundary.
 */

/** Lower-cased comparison key for a sender address or pattern. */
export function normalizeSender(sender: string): string {
  return sender.trim().toLowerCase()
}

const LOCAL_LABEL = "(?:\\*|[\\w!#$%&'*+/=?^_`{|}~-]+)"
const DOMAIN_LABEL = '(?:\\*|[\\w-]+)'

/** Sender-pattern grammar: an address, a whole domain, or `*` labels. */
const SENDER_PATTERN_RE = new RegExp(
  `^(?:@${DOMAIN_LABEL}(?:\\.${DOMAIN_LABEL})*|${LOCAL_LABEL}(?:\\.${LOCAL_LABEL})*@${DOMAIN_LABEL}(?:\\.${DOMAIN_LABEL})*)$`,
)

/** True when the entry is safe to persist and can be displayed again. */
export function isValidRemoteSenderPattern(sender: string): boolean {
  return SENDER_PATTERN_RE.test(normalizeSender(sender))
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function wildcardSource(value: string, wildcard: string): string {
  return value
    .split('.')
    .map((label) => (label === '*' ? wildcard : escapeRegExp(label)))
    .join('\\.')
}

function domainMatchesRule(domain: string, domainRule: string): boolean {
  const source = wildcardSource(domainRule, '[\\w-]+')
  // A leading "@" rule covers the domain itself and all of its subdomains.
  return new RegExp(`(?:^|\\.)${source}$`).test(domain)
}

/** Match one sender against an address, `@domain`, or wildcard entry. */
export function senderMatchesPattern(sender: string, pattern: string): boolean {
  const key = normalizeSender(sender)
  const rule = normalizeSender(pattern)
  if (!key || !isValidRemoteSenderPattern(rule)) return false

  const senderAt = key.lastIndexOf('@')
  if (senderAt <= 0 || senderAt === key.length - 1) return false
  const local = key.slice(0, senderAt)
  const domain = key.slice(senderAt + 1)

  if (rule.startsWith('@')) return domainMatchesRule(domain, rule.slice(1))

  const ruleAt = rule.lastIndexOf('@')
  if (ruleAt <= 0 || ruleAt === rule.length - 1) return false
  const localRule = rule.slice(0, ruleAt)
  const domainRule = rule.slice(ruleAt + 1)
  return (
    new RegExp(`^${wildcardSource(localRule, "[\\w!#$%&'*+/=?^_`{|}~-]+")}$`).test(local) &&
    new RegExp(`^${wildcardSource(domainRule, '[\\w-]+')}$`).test(domain)
  )
}

export type RemoteSenderListResult =
  | { status: 'added' | 'updated'; senders: string[] }
  | { status: 'invalid' | 'duplicate' }

/** Add one entry, rejecting only an exact duplicate rather than overlapping rules. */
export function appendRemoteSenderPattern(
  senders: readonly string[],
  pattern: string,
): RemoteSenderListResult {
  const key = normalizeSender(pattern)
  if (!isValidRemoteSenderPattern(key)) return { status: 'invalid' }
  if (senders.some((sender) => normalizeSender(sender) === key)) {
    return { status: 'duplicate' }
  }
  return { status: 'added', senders: [...senders, key] }
}

/** Replace one entry and persist the list once at the call site. */
export function replaceRemoteSenderPattern(
  senders: readonly string[],
  oldPattern: string,
  newPattern: string,
): RemoteSenderListResult {
  const oldKey = normalizeSender(oldPattern)
  const newKey = normalizeSender(newPattern)
  const index = senders.findIndex((sender) => normalizeSender(sender) === oldKey)
  if (index < 0 || !isValidRemoteSenderPattern(newKey)) return { status: 'invalid' }
  if (senders.some((sender, i) => i !== index && normalizeSender(sender) === newKey)) {
    return { status: 'duplicate' }
  }
  const updated = [...senders]
  updated[index] = newKey
  return { status: 'updated', senders: updated }
}
