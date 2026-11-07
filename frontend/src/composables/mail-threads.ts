import type { EmailSummary } from '~/types/email'

export type EmailThreadRow = EmailSummary

/**
 * Collapse the loaded rows into one list row per conversation.
 *
 * The backend computes thread IDs against the whole local cache, so the count
 * and unread badge can include messages on earlier pages that have not been
 * loaded into this page yet. The visible sender/subject/date still come from
 * the newest message on screen.
 */
export function groupEmailThreads(emails: EmailSummary[]): EmailThreadRow[] {
  const groups = new Map<string, EmailSummary[]>()
  const order: string[] = []

  for (const email of emails) {
    const id =
      email.thread_id?.trim() ||
      `single:${email.account_id ?? ''}\u200b${email.folder_name ?? ''}\u200b${email.uid}`
    let group = groups.get(id)
    if (!group) {
      group = []
      groups.set(id, group)
      order.push(id)
    }
    group.push(email)
  }

  return order.map((id) => {
    const messages = groups.get(id) ?? []
    const lead = messages[0]
    const visibleUnread = messages.filter((email) => !email.is_read).length
    return {
      ...lead,
      thread_count: Math.max(lead.thread_count ?? 0, messages.length),
      thread_unread: Math.max(lead.thread_unread ?? 0, visibleUnread),
    }
  })
}
