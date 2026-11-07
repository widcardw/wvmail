import { groupEmailThreads } from '../src/composables/mail-threads.ts'
import type { EmailSummary } from '../src/types/email.ts'

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

function mail(uid: number, overrides: Partial<EmailSummary> = {}): EmailSummary {
  return {
    uid,
    subject: 'Project',
    from: 'Alice <alice@example.com>',
    from_name: 'Alice',
    from_addr: 'alice@example.com',
    to: ['me@example.com'],
    date: '2026-11-01T10:00:00Z',
    flags: [],
    is_read: false,
    ...overrides,
  }
}

{
  const rows = groupEmailThreads([
    mail(3, { thread_id: 'mid:root@example.com', thread_count: 3, thread_unread: 1 }),
    mail(2, {
      thread_id: 'mid:root@example.com',
      thread_count: 3,
      thread_unread: 1,
      is_read: true,
    }),
    mail(1, { thread_id: 'other', thread_count: 1 }),
  ])
  check('collapses the same thread into one row', rows.length === 2)
  check('keeps the newest message as the row lead', rows[0]?.uid === 3)
  check('keeps the backend thread count', rows[0]?.thread_count === 3)
  check('keeps the backend unread count', rows[0]?.thread_unread === 1)
  check('keeps both visible members for selection', rows[0]?.thread_messages.length === 2)
}

{
  const rows = groupEmailThreads([
    mail(2, { thread_id: 'mid:root@example.com', thread_count: 3 }),
    mail(1, { thread_id: 'mid:root@example.com', thread_count: 3, is_read: true }),
  ])
  check('falls back to visible unread rows', rows[0]?.thread_unread === 1)
}

{
  const rows = groupEmailThreads([mail(1), mail(2)])
  check('messages without a thread ID stay separate', rows.length === 2)
  check(
    'single-message rows count as one',
    rows.every((row) => row.thread_count === 1),
  )
}

if (fail > 0) {
  console.error(`\n${fail} failed, ${pass} passed`)
  process.exit(1)
}
console.log(`\n${pass} passed`)
