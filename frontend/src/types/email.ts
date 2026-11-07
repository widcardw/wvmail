// TypeScript types matching Go email models
// The Go field names are PascalCase and map directly to JS property names.

// export interface EmailAccount {
//   ID: string
//   Name: string
//   Email: string
//   Provider: EmailProvider
//   IMAPHost: string
//   IMAPPort: number
//   IMAPUser: string
//   IMAPPass: string
//   SMTPHost: string
//   SMTPPort: number
//   SMTPUser: string
//   SMTPPass: string
//   UseTLS: boolean
//   SyncInterval: number
// }

export { EmailAccount } from '#/wvmail/services'

/** Messages per page, shared by the mail list and the search results. */
export const PAGE_SIZE = 20

export interface EmailSummary {
  uid: number
  message_id?: string
  in_reply_to?: string[]
  references?: string[]
  subject: string
  from: string
  from_name: string
  from_addr: string
  to: string[]
  date: string
  flags: string[]
  is_read: boolean
  is_bulk?: boolean
  thread_id?: string
  thread_count?: number
  thread_unread?: number
  tag_ids?: string[]
  account_id?: string
  folder_name?: string
}

export type EmailSearchField = 'from' | 'to' | 'content'

export interface EmailSearchFilter {
  field: EmailSearchField
  value: string
}

export interface EmailDetail {
  uid: number
  message_id?: string
  in_reply_to?: string[]
  references?: string[]
  thread_id?: string
  thread_count?: number
  thread_unread?: number
  subject: string
  from: string
  from_name: string
  from_addr: string
  to: string[]
  cc: string[]
  date: string
  body_text: string
  body_html: string
  /** True when the MIME body was text/plain and no HTML alternative was used. */
  body_is_plain_text: boolean
  flags: string[]
  attachments: AttachmentInfo[]
  is_suspicious: boolean
}

export interface AttachmentInfo {
  filename: string
  mime_type: string
  size: number
  part_path: number[]
  is_inline: boolean
}

export interface FolderInfo {
  name: string
  delim: string
  attrs: string[]
}

export interface EmailPageResponse {
  emails: EmailSummary[]
  total: number
  /** True when the backend served this page from its cache without dialing IMAP. */
  from_cache?: boolean
  /**
   * The mailbox's UIDNEXT at fetch time; 0 when unknown. UIDs only grow, so a
   * later STATUS reporting the same value means nothing has arrived since — see
   * `revalidateOpenFolder` in stores/accounts.ts.
   */
  uid_next?: number
}

/**
 * One folder's counters as the server reported them (STATUS MESSAGES UIDNEXT
 * UNSEEN). `unseen` drives the sidebar badge; `messages` + `uid_next` are the
 * change signal that keeps opening a folder from re-fetching an unchanged list.
 */
export interface FolderStatus {
  name: string
  attrs: string[]
  messages: number
  uid_next: number
  unseen: number
}

export type EmailProvider = 'gmail' | 'outlook' | '163' | 'qq' | 'custom'

export interface ProviderDefaults {
  imap_host: string
  imap_port: number
  imap_use_tls: boolean
  smtp_host: string
  smtp_port: number
}

export const EMAIL_PROVIDER_DEFAULTS: Record<EmailProvider, ProviderDefaults> = {
  gmail: {
    imap_host: 'imap.gmail.com',
    imap_port: 993,
    imap_use_tls: true,
    smtp_host: 'smtp.gmail.com',
    smtp_port: 587,
  },
  outlook: {
    imap_host: 'outlook.office365.com',
    imap_port: 993,
    imap_use_tls: true,
    smtp_host: 'smtp-mail.outlook.com',
    smtp_port: 587,
  },
  '163': {
    imap_host: 'imap.163.com',
    imap_port: 993,
    imap_use_tls: true,
    smtp_host: 'smtp.163.com',
    smtp_port: 465,
  },
  qq: {
    imap_host: 'imap.qq.com',
    imap_port: 993,
    imap_use_tls: true,
    smtp_host: 'smtp.qq.com',
    smtp_port: 465,
  },
  custom: {
    imap_host: '',
    imap_port: 993,
    imap_use_tls: true,
    smtp_host: '',
    smtp_port: 587,
  },
}

export const EMAIL_PROVIDER_LABELS: Record<EmailProvider, string> = {
  gmail: 'Gmail',
  outlook: 'Outlook / Hotmail',
  '163': '163',
  qq: 'QQ 邮箱',
  custom: '自定义',
}

/**
 * Detect provider from an email address domain.
 */
export function detectProvider(email: string): EmailProvider {
  const domain = email.split('@')[1]?.toLowerCase() ?? ''
  if (domain.includes('gmail.com')) return 'gmail'
  if (
    domain.includes('outlook.com') ||
    domain.includes('hotmail.com') ||
    domain.includes('live.com')
  )
    return 'outlook'
  if (domain.includes('163.com') || domain.includes('126.com')) return '163'
  if (domain.includes('qq.com') || domain.includes('foxmail.com')) return 'qq'
  return 'custom'
}

export { getFolderDisplay, compareFolders } from '../composables/folder-semantics'
export type { FolderDisplay } from '../composables/folder-semantics'
