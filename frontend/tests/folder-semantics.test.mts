import {
  accountUnreadCount,
  adjustUnreadCounts,
  getFolderDisplay,
} from '../src/composables/folder-semantics.ts'
import { buildFolderTree } from '../src/composables/folder-tree.ts'

let pass = 0
let fail = 0

function check(name: string, cond: boolean) {
  if (cond) {
    pass++
    console.log(`  PASS  ${name}`)
  } else {
    fail++
    console.error(`  FAIL  ${name}`)
  }
}

console.log('folder semantics')

const allMail = getFolderDisplay('[Gmail]/所有邮件', 'zh-CN', ['\\All', '\\HasNoChildren'])
check('special-use All Mail is labeled as all mail, not archive', allMail.label === '所有邮件')
check('All Mail sorts before Spam and Trash', allMail.rank < getFolderDisplay('Junk').rank)

const important = getFolderDisplay('[Gmail]/重要', 'en-US', ['\\Important'])
check('localized Gmail Important uses the special-use attribute', important.label === 'Important')

const archive = getFolderDisplay('Archive', 'zh-CN')
check('a real Archive folder keeps the archive label', archive.label === '归档')

const tree = buildFolderTree(
  [
    { name: '[Gmail]/所有邮件', attrs: ['\\All'], delim: '/' },
    { name: '[Gmail]/已发邮件', attrs: ['\\Sent'], delim: '/' },
  ],
  (segment, attrs) => getFolderDisplay(segment, 'zh-CN', attrs),
)
check(
  'folder tree passes special-use attributes to display resolution',
  tree.map((node) => node.label).join('|') === '发件箱|所有邮件',
)

const gmailStatuses = {
  INBOX: { name: 'INBOX', attrs: [], unseen: 1 },
  important: { name: '[Gmail]/重要', attrs: ['\\Important'], unseen: 1 },
  all: { name: '[Gmail]/所有邮件', attrs: ['\\All'], unseen: 1 },
}
check('Gmail account badge counts All Mail once', accountUnreadCount(gmailStatuses) === 1)

check(
  'Gmail falls back to Inbox when All Mail is unavailable',
  accountUnreadCount({
    INBOX: { name: 'INBOX', attrs: [], unseen: 2 },
    important: { name: '[Gmail]/重要', attrs: ['\\Important'], unseen: 2 },
  }) === 2,
)

check(
  'non-Gmail account badge still sums distinct folders',
  accountUnreadCount({
    INBOX: { name: 'INBOX', unseen: 1 },
    Archive: { name: 'Archive', unseen: 2 },
  }) === 3,
)

adjustUnreadCounts(gmailStatuses, 'INBOX', -1)
check(
  'marking an Inbox message read also updates the All Mail counter',
  gmailStatuses.INBOX.unseen === 0 && gmailStatuses.all.unseen === 0,
)

const spamStatuses = {
  spam: { name: '[Gmail]/垃圾邮件', attrs: ['\\Junk'], unseen: 1 },
  all: { name: '[Gmail]/所有邮件', attrs: ['\\All'], unseen: 0 },
}
adjustUnreadCounts(spamStatuses, 'spam', -1)
check(
  'Spam is not part of All Mail and does not decrement it',
  spamStatuses.spam.unseen === 0 && spamStatuses.all.unseen === 0,
)

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
