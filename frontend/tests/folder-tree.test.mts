import { buildFolderTree } from '../src/composables/folder-tree.ts'
import type { FolderTreeMailbox } from '../src/composables/folder-tree.ts'

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

function folder(name: string, attrs: string[] = [], delim = '/'): FolderTreeMailbox {
  return { name, attrs, delim }
}

console.log('nested IMAP folder tree')

const outlook = buildFolderTree([
  folder('Inbox', [], '/'),
  folder('同步问题', [], '/'),
  folder('同步问题/本地故障'),
  folder('同步问题/冲突'),
  folder('同步问题/服务器故障'),
])
const syncIssues = outlook.find((node) => node.fullName === '同步问题')
check('keeps unrelated folders at account root', outlook.some((node) => node.fullName === 'Inbox'))
check('groups Outlook sync issue subfolders beneath their parent', syncIssues?.children.length === 3)
check(
  'preserves exact mailbox paths for children',
  syncIssues?.children.map((node) => node.fullName).sort().join('|') ===
    ['同步问题/本地故障', '同步问题/冲突', '同步问题/服务器故障'].sort().join('|'),
)
check('parent mailbox itself remains selectable', syncIssues?.selectable === true)
check('child labels omit the repeated parent path', syncIssues?.children[0]?.label === '冲突')

const noSelectParent = buildFolderTree([
  folder('项目', ['\\Noselect', '\\HasChildren']),
  folder('项目/进展/本周'),
])
check('retains non-selectable parent nodes when they have children', noSelectParent[0]?.selectable === false)
check('synthesizes missing intermediate parents', noSelectParent[0]?.children[0]?.fullName === '项目/进展')
check(
  'synthesized parents are not selectable but expose descendants',
  noSelectParent[0]?.children[0]?.selectable === false &&
    noSelectParent[0]?.children[0]?.children[0]?.fullName === '项目/进展/本周',
)

const dotDelimited = buildFolderTree([folder('Projects.Current', [], '.')])
check('uses the server-provided hierarchy delimiter', dotDelimited[0]?.children[0]?.fullName === 'Projects.Current')

const gmail = buildFolderTree([folder('[Gmail]/All Mail'), folder('[Gmail]/Sent Mail')])
check('keeps Gmail system namespace folders flat', gmail.length === 2)

const hidden = buildFolderTree([
  folder('Removed', ['\\NonExistent']),
  folder('Empty branch', ['\\Noselect']),
])
check('omits nonexistent and empty non-selectable folders', hidden.length === 0)

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
