import type { EmailAddress } from '#/wvmail/services'

/**
 * Split a user-typed recipient line without cutting inside quotes or angle
 * brackets. The compose field accepts both comma and semicolon separators.
 */
export function splitRecipientText(value: string): string[] {
  const result: string[] = []
  let current = ''
  let quoted = false
  let angled = false
  for (const char of value) {
    if (char === '"' && !angled) {
      quoted = !quoted
      current += char
      continue
    }
    if (char === '<' && !quoted) angled = true
    if (char === '>' && !quoted) angled = false
    if ((char === ',' || char === ';') && !quoted && !angled) {
      if (current.trim()) result.push(current.trim())
      current = ''
      continue
    }
    current += char
  }
  if (current.trim()) result.push(current.trim())
  return result
}

/** Parse display names and bare addresses into the backend's address shape. */
export function parseRecipientText(value: string): EmailAddress[] {
  const result: EmailAddress[] = []
  for (const item of splitRecipientText(value)) {
    const match = item.match(/^\s*(?:"?([^"<]*?)"?\s*)?<([^<>]+)>\s*$/)
    if (match) {
      const name = (match[1] ?? '').trim()
      const address = match[2].trim()
      if (address) result.push({ Name: name, Address: address } as EmailAddress)
      continue
    }
    if (item.includes('@')) result.push({ Name: '', Address: item } as EmailAddress)
  }
  return result
}

export function formatRecipientText(addresses: EmailAddress[] | undefined): string {
  return (addresses ?? [])
    .map((address) => (address.Name ? `${address.Name} <${address.Address}>` : address.Address))
    .join(', ')
}

export function isImageFilename(name: string): boolean {
  return /\.(avif|bmp|gif|jpe?g|png|svg|webp)$/i.test(name)
}
