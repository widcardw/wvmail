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
  subject: string
  from: string
  from_name: string
  from_addr: string
  to: string[]
  date: string
  flags: string[]
  is_read: boolean
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
  subject: string
  from: string
  from_name: string
  from_addr: string
  to: string[]
  cc: string[]
  date: string
  body_text: string
  body_html: string
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

/**
 * Canonical folder categories with sort order.
 * Lower rank = appears first in the sidebar.
 */
export interface FolderCategory {
  icon: string
  /** Canonical display name in Chinese (used when locale is zh-CN) */
  labelCn: string
  /** Canonical display name in English */
  labelEn: string
  /** Sort rank: 0=Inbox, 1=Sent, 2=Drafts, 3=Trash, 4=Spam, 99=other */
  rank: number
}

const FOLDER_CATEGORIES: Array<{ patterns: string[]; category: FolderCategory }> = [
  {
    patterns: ['inbox'],
    category: { icon: 'ri:inbox-2-line', labelCn: '收件箱', labelEn: 'Inbox', rank: 0 },
  },
  {
    patterns: [
      'sent',
      'sent mail',
      'sent messages',
      'sent items',
      '已发送',
      '已发送邮件',
      '[gmail]/sent mail',
    ],
    category: { icon: 'ri:send-plane-line', labelCn: '发件箱', labelEn: 'Sent', rank: 1 },
  },
  {
    patterns: ['drafts', '草稿', '草稿箱', '[gmail]/drafts'],
    category: { icon: 'ri:draft-line', labelCn: '草稿', labelEn: 'Drafts', rank: 2 },
  },
  {
    patterns: [
      'trash',
      'bin',
      'deleted',
      'deleted items',
      'deleted messages',
      '已删除',
      '垃圾箱',
      '[gmail]/trash',
      '[gmail]/bin',
    ],
    category: { icon: 'ri:delete-bin-5-line', labelCn: '已删除', labelEn: 'Trash', rank: 3 },
  },
  {
    patterns: [
      'spam',
      'junk',
      'spam folder',
      'junk mail',
      'junk folder',
      '垃圾邮件',
      '[gmail]/spam',
    ],
    category: { icon: 'ri:spam-2-line', labelCn: '垃圾邮件', labelEn: 'Spam', rank: 4 },
  },
  {
    patterns: [
      'archive',
      'all mail',
      'allmail',
      '归档',
      '全部邮件',
      '[gmail]/all mail',
      '[gmail]/archive',
    ],
    category: { icon: 'ri:archive-2-line', labelCn: '归档', labelEn: 'Archive', rank: 5 },
  },
  {
    patterns: ['starred', 'flagged', 'star', '已加星标', '星标邮件', '[gmail]/starred'],
    category: { icon: 'ri:star-line', labelCn: '星标邮件', labelEn: 'Starred', rank: 6 },
  },
  {
    patterns: ['important', '重要', '重要邮件', '[gmail]/important'],
    category: {
      icon: 'ri:notification-3-line',
      labelCn: '重要邮件',
      labelEn: 'Important',
      rank: 7,
    },
  },
]

/**
 * Returns sorted display info for an IMAP folder.
 * - `icon`: emoji icon
 * - `label`: user-friendly name (strips Gmail-style "[Gmail]/" prefix)
 * - `rank`: sort priority (lower = earlier)
 */
export function getFolderDisplay(
  folderName: string,
  locale: 'zh-CN' | 'en-US' = 'en-US',
): {
  icon: string
  label: string
  rank: number
} {
  const lower = folderName.toLowerCase().trim()
  for (const entry of FOLDER_CATEGORIES) {
    for (const pattern of entry.patterns) {
      if (lower === pattern || lower.endsWith('/' + pattern)) {
        // const displayName = folderName.replace(/^\[.*?\]\//, '')
        const label = locale === 'zh-CN' ? entry.category.labelCn : entry.category.labelEn
        return { icon: entry.category.icon, label, rank: entry.category.rank }
      }
    }
  }
  // No match — generic folder, highest rank so it sorts last
  const displayName = folderName.replace(/^\[.*?\]\//, '')
  return { icon: 'ri:folder-2-line', label: displayName, rank: 99 }
}

/**
 * Sort comparator for FolderInfo: standard folders first (by rank), then
 * the rest alphabetically.
 */
export function compareFolders(
  a: FolderInfo,
  b: FolderInfo,
  locale: 'zh-CN' | 'en-US' = 'en-US',
): number {
  const da = getFolderDisplay(a.name, locale)
  const db = getFolderDisplay(b.name, locale)
  if (da.rank !== db.rank) return da.rank - db.rank
  return da.label.localeCompare(db.label)
}
