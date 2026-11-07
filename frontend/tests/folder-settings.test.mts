import {
  applyFolderSettings,
  folderDisplayOverride,
  orderFolderNodes,
} from '../src/composables/folder-settings.ts'
import type { FolderTreeNode } from '../src/composables/folder-tree.ts'

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

function folder(
  fullName: string,
  label: string,
  children: FolderTreeNode[] = [],
  selectable = true,
): FolderTreeNode {
  return {
    fullName,
    label,
    icon: 'ri:folder-2-line',
    rank: 99,
    selectable,
    children,
  }
}

console.log('folder display settings')

const tree = [
  folder('INBOX', 'Inbox'),
  folder('Projects', 'Projects', [folder('Projects/2026', '2026')], false),
  folder('Spam', 'Spam'),
]

const overridden = applyFolderSettings(tree, [
  { Name: 'INBOX', DisplayName: '工作收件箱', Icon: 'ri:briefcase-line' },
])
check('custom name replaces the automatic label', overridden[0]?.label === '工作收件箱')
check('custom icon replaces the automatic icon', overridden[0]?.icon === 'ri:briefcase-line')
check('folders without a rule keep their automatic display', overridden[2]?.label === 'Spam')

const hidden = applyFolderSettings(tree, [{ Name: 'Projects', Hidden: true }])
check('hidden folder is removed', !hidden.some((node) => node.fullName === 'Projects'))
check(
  'hidden folder takes its descendants with it',
  !hidden.some((node) => node.fullName === 'Projects/2026'),
)

const hiddenChild = applyFolderSettings(tree, [{ Name: 'Projects/2026', Hidden: true }])
check(
  'non-selectable parent disappears when all children are hidden',
  !hiddenChild.some((node) => node.fullName === 'Projects'),
)

const whitespace = applyFolderSettings(tree, [{ Name: 'INBOX', DisplayName: '   ', Icon: '   ' }])
check('blank display name keeps the automatic label', whitespace[0]?.label === 'Inbox')
check('blank icon keeps the automatic icon', whitespace[0]?.icon === 'ri:folder-2-line')

const override = folderDisplayOverride(
  [{ Name: 'INBOX', Hidden: true, DisplayName: ' 收件箱 ', Icon: ' ri:inbox-2-line ' }],
  'INBOX',
)
check('override reports hidden state', override.hidden)
check('override trims a custom label', override.label === '收件箱')
check('override trims a custom icon', override.icon === 'ri:inbox-2-line')

check(
  'saved order arranges visible folders',
  applyFolderSettings(tree, undefined, ['Spam', 'INBOX'])
    .map((node) => node.fullName)
    .join('|') === 'Spam|INBOX|Projects',
)

check(
  'unknown order entries are ignored',
  orderFolderNodes(tree, ['Gone', 'Spam', 'INBOX'])
    .map((node) => node.fullName)
    .join('|') === 'Spam|INBOX|Projects',
)

check(
  'unlisted folders keep their automatic order after the listed ones',
  orderFolderNodes(tree, ['INBOX'])
    .map((node) => node.fullName)
    .join('|') === 'INBOX|Projects|Spam',
)

const nested = [
  folder('Work', 'Work', [folder('Work/Bills', 'Bills'), folder('Work/Travel', 'Travel')], false),
]
check(
  'order applies among siblings inside a parent',
  orderFolderNodes(nested, ['Work/Travel', 'Work/Bills'])[0]
    ?.children.map((node) => node.fullName)
    .join('|') === 'Work/Travel|Work/Bills',
)
check(
  'a child can never leave its parent',
  orderFolderNodes(nested, ['Work/Bills', 'Work', 'Work/Travel'])[0]
    ?.children.map((node) => node.fullName)
    .join('|') === 'Work/Bills|Work/Travel',
)

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
