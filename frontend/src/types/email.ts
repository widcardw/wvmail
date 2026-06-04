// TypeScript types matching Go email models
// The Go field names are PascalCase and map directly to JS property names.

export interface EmailAccount {
  ID: string
  Name: string
  Email: string
  Provider: EmailProvider
  IMAPHost: string
  IMAPPort: number
  IMAPUser: string
  IMAPPass: string
  SMTPHost: string
  SMTPPort: number
  SMTPUser: string
  SMTPPass: string
  UseTLS: boolean
}

export interface EmailSummary {
  uid: number
  subject: string
  from: string
  date: string
  flags: string[]
  is_read: boolean
}

export interface EmailDetail {
  uid: number
  subject: string
  from: string
  to: string[]
  cc: string[]
  date: string
  body_text: string
  body_html: string
  flags: string[]
}

export interface FolderInfo {
  name: string
  delim: string
  attrs: string[]
}

export type EmailProvider =
  | 'gmail'
  | 'outlook'
  | '163'
  | 'qq'
  | 'custom'

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
  '163': '163 邮箱',
  qq: 'QQ 邮箱',
  custom: '自定义',
}

/**
 * Detect provider from an email address domain.
 */
export function detectProvider(email: string): EmailProvider {
  const domain = email.split('@')[1]?.toLowerCase() ?? ''
  if (domain.includes('gmail.com')) return 'gmail'
  if (domain.includes('outlook.com') || domain.includes('hotmail.com') || domain.includes('live.com')) return 'outlook'
  if (domain.includes('163.com') || domain.includes('126.com')) return '163'
  if (domain.includes('qq.com') || domain.includes('foxmail.com')) return 'qq'
  return 'custom'
}

/**
 * Standard IMAP folder name mapping to emoji icon + user-friendly name.
 * Matches common English names and Gmail's bracketed names.
 */
const FOLDER_ICON_MAP: Array<{ patterns: string[]; icon: string; labelKey?: string }> = [
  { patterns: ['inbox'], icon: '📥' },
  { patterns: ['sent', 'sent mail', 'sent messages', '已发送', '[gmail]/sent mail'], icon: '📤' },
  { patterns: ['drafts', '草稿', '[gmail]/drafts'], icon: '📝' },
  { patterns: ['trash', 'deleted', 'deleted messages', 'bin', '垃圾箱', '已删除', '[gmail]/trash', '[gmail]/bin'], icon: '🗑' },
  { patterns: ['spam', 'junk', '垃圾邮件', '[gmail]/spam'], icon: '⚠️' },
  { patterns: ['archive', 'all mail', '归档', '全部邮件', '[gmail]/all mail', '[gmail]/archive'], icon: '📦' },
  { patterns: ['starred', 'flagged', '已加星标', '[gmail]/starred'], icon: '⭐' },
  { patterns: ['important', '重要', '[gmail]/important'], icon: '🔔' },
]

/**
 * Returns emoji icon + user-friendly name for an IMAP folder.
 * Falls back to a generic folder icon and the original name.
 */
export function getFolderDisplay(folderName: string): { icon: string; label: string } {
  const lower = folderName.toLowerCase().trim()
  for (const entry of FOLDER_ICON_MAP) {
    for (const pattern of entry.patterns) {
      if (lower === pattern || lower.endsWith('/' + pattern)) {
        // Strip Gmail-style prefix for display: "[Gmail]/Sent Mail" → "Sent Mail"
        const displayName = folderName.replace(/^\[.*?\]\//, '')
        return { icon: entry.icon, label: displayName }
      }
    }
  }
  // No match — use generic icon
  const displayName = folderName.replace(/^\[.*?\]\//, '')
  return { icon: '📁', label: displayName }
}
