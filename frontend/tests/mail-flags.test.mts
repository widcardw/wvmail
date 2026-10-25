import { hasMailFlag, isFlagged, withMailFlag } from '../src/composables/mail-flags.ts'

let pass = 0
let fail = 0

function check(name: string, cond: boolean) {
  if (cond) {
    pass++
    console.log(`  ok   ${name}`)
  } else {
    fail++
    console.log(`  FAIL ${name}`)
  }
}

console.log('mail flags')

check('flagged is recognised regardless of case', isFlagged(['\\seen', '\\FLAGGED']))
check('other flags are not mistaken for starred', !isFlagged(['\\Seen', '\\Answered']))
check('an empty flag list is not starred', !isFlagged([]))
check('an exact flag lookup is case-insensitive', hasMailFlag(['$Forwarded'], '$forwarded'))

check(
  'adding a flag keeps the existing order',
  withMailFlag(['\\Seen'], '\\Flagged', true).join(',') === '\\Seen,\\Flagged',
)
check(
  'adding an existing flag does not duplicate it',
  withMailFlag(['\\Seen', '\\FLAGGED'], '\\Flagged', true).join(',') === '\\Seen,\\FLAGGED',
)
check(
  'removing a flag keeps the others',
  withMailFlag(['\\Seen', '\\Flagged', '\\Answered'], '\\Flagged', false).join(',') ===
    '\\Seen,\\Answered',
)
check(
  'removing an absent flag changes nothing',
  withMailFlag(['\\Seen'], '\\Flagged', false).join(',') === '\\Seen',
)

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
