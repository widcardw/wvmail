export interface FolderDisplay {
  icon: string
  label: string
  rank: number
}

export interface FolderUnreadStatus {
  name: string
  attrs?: string[]
  unseen?: number
}

type FolderLocale = 'zh-CN' | 'en-US'

const INBOX: FolderDisplay = {
  icon: 'ri:inbox-2-line',
  label: 'Inbox',
  rank: 0,
}
const STARRED: FolderDisplay = {
  icon: 'ri:star-line',
  label: 'Starred',
  rank: 1,
}
const IMPORTANT: FolderDisplay = {
  icon: 'ri:notification-3-line',
  label: 'Important',
  rank: 2,
}
const SENT: FolderDisplay = {
  icon: 'ri:send-plane-line',
  label: 'Sent',
  rank: 3,
}
const DRAFTS: FolderDisplay = {
  icon: 'ri:draft-line',
  label: 'Drafts',
  rank: 4,
}
const ALL_MAIL: FolderDisplay = {
  icon: 'ri:mail-line',
  label: 'All Mail',
  rank: 5,
}
const SPAM: FolderDisplay = {
  icon: 'ri:spam-2-line',
  label: 'Spam',
  rank: 6,
}
const TRASH: FolderDisplay = {
  icon: 'ri:delete-bin-5-line',
  label: 'Trash',
  rank: 7,
}
const ARCHIVE: FolderDisplay = {
  icon: 'ri:archive-2-line',
  label: 'Archive',
  rank: 8,
}
const VIRUS: FolderDisplay = {
  icon: 'ri:virus-line',
  label: 'Virus',
  rank: 9,
}

const LOCALIZED_LABELS: Record<string, Record<FolderLocale, string>> = {
  inbox: { 'zh-CN': '收件箱', 'en-US': 'Inbox' },
  starred: { 'zh-CN': '星标邮件', 'en-US': 'Starred' },
  important: { 'zh-CN': '重要邮件', 'en-US': 'Important' },
  sent: { 'zh-CN': '发件箱', 'en-US': 'Sent' },
  drafts: { 'zh-CN': '草稿', 'en-US': 'Drafts' },
  allMail: { 'zh-CN': '所有邮件', 'en-US': 'All Mail' },
  spam: { 'zh-CN': '垃圾邮件', 'en-US': 'Spam' },
  trash: { 'zh-CN': '已删除', 'en-US': 'Trash' },
  archive: { 'zh-CN': '归档', 'en-US': 'Archive' },
  virus: { 'zh-CN': '病毒邮件', 'en-US': 'Virus' },
}

const SPECIAL_USE: Record<string, FolderDisplay> = {
  '\\all': ALL_MAIL,
  '\\important': IMPORTANT,
  '\\flagged': STARRED,
  '\\sent': SENT,
  '\\drafts': DRAFTS,
  '\\junk': SPAM,
  '\\trash': TRASH,
  '\\archive': ARCHIVE,
}

const NAME_PATTERNS: Array<{ patterns: string[]; display: FolderDisplay }> = [
  { patterns: ['inbox', '收件箱'], display: INBOX },
  {
    patterns: ['starred', 'flagged', 'star', '已加星标', '星标邮件', '[gmail]/starred'],
    display: STARRED,
  },
  {
    patterns: ['important', '重要', '重要邮件', '[gmail]/important'],
    display: IMPORTANT,
  },
  {
    patterns: [
      'sent',
      'sent mail',
      'sent messages',
      'sent items',
      '已发送',
      '已发送邮件',
      '已发邮件',
      '[gmail]/sent mail',
    ],
    display: SENT,
  },
  {
    patterns: ['drafts', '草稿', '草稿箱', '[gmail]/drafts'],
    display: DRAFTS,
  },
  {
    patterns: ['all mail', 'allmail', '全部邮件', '所有邮件', '[gmail]/all mail'],
    display: ALL_MAIL,
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
    display: SPAM,
  },
  {
    patterns: [
      'trash',
      'bin',
      'deleted',
      'deleted items',
      'deleted messages',
      '已删除',
      '已删除邮件',
      '已删邮件',
      '垃圾箱',
      '[gmail]/trash',
      '[gmail]/bin',
    ],
    display: TRASH,
  },
  {
    patterns: ['archive', '归档', '存档', '[gmail]/archive'],
    display: ARCHIVE,
  },
  {
    patterns: ['virus', '病毒', '病毒邮件', '病毒文件夹'],
    display: VIRUS,
  },
]

function localized(display: FolderDisplay, locale: FolderLocale): FolderDisplay {
  const key = Object.entries({
    inbox: INBOX,
    starred: STARRED,
    important: IMPORTANT,
    sent: SENT,
    drafts: DRAFTS,
    allMail: ALL_MAIL,
    spam: SPAM,
    trash: TRASH,
    archive: ARCHIVE,
    virus: VIRUS,
  }).find(([, candidate]) => candidate === display)?.[0]
  return key ? { ...display, label: LOCALIZED_LABELS[key][locale] } : display
}

function hasAttr(status: FolderUnreadStatus, attr: string): boolean {
  return (status.attrs ?? []).some((candidate) => candidate.toLowerCase() === attr)
}

function isGmailView(status: FolderUnreadStatus): boolean {
  return status.name.toLowerCase().startsWith('[gmail]/') || hasAttr(status, '\\all')
}

/**
 * Returns display info for an IMAP folder. Special-use attributes win over the
 * mailbox name because Gmail localizes every system folder name.
 */
export function getFolderDisplay(
  folderName: string,
  locale: FolderLocale = 'en-US',
  attrs: string[] = [],
): FolderDisplay {
  for (const attr of attrs) {
    const display = SPECIAL_USE[attr.toLowerCase()]
    if (display) return localized(display, locale)
  }

  const lower = folderName.toLowerCase().trim()
  for (const entry of NAME_PATTERNS) {
    for (const pattern of entry.patterns) {
      if (lower === pattern || lower.endsWith(`/${pattern}`)) {
        return localized(entry.display, locale)
      }
    }
  }

  return {
    icon: 'ri:folder-2-line',
    label: folderName.replace(/^\[.*?\]\//, ''),
    rank: 99,
  }
}

/** Sort comparator: standard folders first, then the rest alphabetically. */
export function compareFolders(
  a: { name: string; attrs?: string[] },
  b: { name: string; attrs?: string[] },
  locale: FolderLocale = 'en-US',
): number {
  const da = getFolderDisplay(a.name, locale, a.attrs)
  const db = getFolderDisplay(b.name, locale, b.attrs)
  if (da.rank !== db.rank) return da.rank - db.rank
  return da.label.localeCompare(db.label)
}

/**
 * The account badge must count each message once. Gmail exposes INBOX,
 * Important, Starred and All Mail as overlapping views of the same messages;
 * All Mail is the unique superset, so it becomes the canonical counter.
 */
export function accountUnreadCount(
  statuses: Record<string, FolderUnreadStatus | undefined> | undefined,
): number {
  const folders = Object.values(statuses ?? {}).filter(
    (status): status is FolderUnreadStatus => status !== undefined,
  )
  if (folders.length === 0) return 0

  const allMail = folders.find((status) => hasAttr(status, '\\all'))
  if (allMail) return allMail.unseen ?? 0

  if (!folders.some(isGmailView)) {
    return folders.reduce((total, status) => total + (status.unseen ?? 0), 0)
  }

  const inbox = folders.find((status) => status.name.toLowerCase() === 'inbox')
  if (inbox) return inbox.unseen ?? 0
  return Math.max(...folders.map((status) => status.unseen ?? 0))
}

/**
 * Apply an optimistic unread change. Any message outside Spam/Trash that is
 * marked read also leaves Gmail's All Mail unread set, even when the open row
 * came from a label such as INBOX or Important.
 */
export function adjustUnreadCounts(
  statuses: Record<string, FolderUnreadStatus | undefined> | undefined,
  folder: string,
  delta: number,
): void {
  if (!statuses) return
  const source = statuses[folder]
  if (source) {
    source.unseen = Math.max(0, (source.unseen ?? 0) + delta)
  }

  const allMail = Object.values(statuses).find(
    (status): status is FolderUnreadStatus =>
      status !== undefined && status.name !== folder && hasAttr(status, '\\all'),
  )
  if (
    !allMail ||
    hasAttr(source ?? { name: folder }, '\\trash') ||
    hasAttr(source ?? { name: folder }, '\\junk')
  ) {
    return
  }
  allMail.unseen = Math.max(0, (allMail.unseen ?? 0) + delta)
}
