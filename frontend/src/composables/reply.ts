import type { EmailAddress } from '#/wvmail/services'
import type { EmailDetail } from '~/types/email'
import { parseRecipientText } from './compose.ts'

export type ReplyMode = 'reply' | 'reply-all' | 'forward'

export interface ReplyEnvelope {
  To: EmailAddress[]
  Cc: EmailAddress[]
  Bcc: EmailAddress[]
  Subject: string
  InReplyTo: string
  References: string[]
}

export function parseAddressHeaders(values: string[] | undefined): EmailAddress[] {
  return (values ?? []).flatMap((value) => parseRecipientText(value))
}

export function senderAddress(detail: EmailDetail): EmailAddress | null {
  const parsed = parseRecipientText(detail.from_addr || detail.from || '')[0]
  if (!parsed?.Address) return null
  return {
    Name: detail.from_name || parsed.Name || '',
    Address: parsed.Address,
  } as EmailAddress
}

function uniqueAddresses(addresses: EmailAddress[], excluded: string[] = []): EmailAddress[] {
  const seen = new Set(
    excluded.map((address) => address.trim().toLowerCase()).filter((address) => address.length > 0),
  )
  const result: EmailAddress[] = []
  for (const address of addresses) {
    const key = address.Address.trim().toLowerCase()
    if (!key || seen.has(key)) continue
    seen.add(key)
    result.push({ Name: address.Name, Address: address.Address } as EmailAddress)
  }
  return result
}

function uniqueMessageIDs(values: string[]): string[] {
  const seen = new Set<string>()
  const result: string[] = []
  for (const value of values) {
    const trimmed = value.trim()
    if (!trimmed) continue
    const key = trimmed.replace(/^</, '').replace(/>$/, '').toLowerCase()
    if (!key || seen.has(key)) continue
    seen.add(key)
    result.push(trimmed)
  }
  return result
}

export function subjectWithPrefix(subject: string, mode: ReplyMode): string {
  const value = subject.trim()
  if (mode === 'forward') {
    return /^(?:fwd?|forward)\s*:/i.test(value) ? value : `Fwd: ${value}`
  }
  return /^re\s*:/i.test(value) ? value : `Re: ${value}`
}

export function buildReplyEnvelope(
  detail: EmailDetail,
  mode: ReplyMode,
  accountEmail: string,
): ReplyEnvelope {
  const sender = senderAddress(detail)
  const messageID = (detail.message_id ?? '').trim()
  const inReplyTo = mode === 'forward' ? '' : messageID
  const references =
    mode === 'forward'
      ? []
      : uniqueMessageIDs([...(detail.references ?? []), ...(detail.in_reply_to ?? []), messageID])
  const subject = subjectWithPrefix(detail.subject, mode)

  if (mode === 'forward') {
    return {
      To: [],
      Cc: [],
      Bcc: [],
      Subject: subject,
      InReplyTo: '',
      References: [],
    }
  }

  const to = sender ? [sender] : []
  if (mode === 'reply') {
    return {
      To: to,
      Cc: [],
      Bcc: [],
      Subject: subject,
      InReplyTo: inReplyTo,
      References: references,
    }
  }

  const originalRecipients = parseAddressHeaders([...(detail.to ?? []), ...(detail.cc ?? [])])
  return {
    To: to,
    Cc: uniqueAddresses(originalRecipients, [accountEmail, ...(sender ? [sender.Address] : [])]),
    Bcc: [],
    Subject: subject,
    InReplyTo: inReplyTo,
    References: references,
  }
}

export function escapeHTML(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}
