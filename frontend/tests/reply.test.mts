import {
  buildReplyEnvelope,
  escapeHTML,
  parseAddressHeaders,
  senderAddress,
  subjectWithPrefix,
} from '../src/composables/reply.ts'

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

const detail = {
  uid: 1,
  message_id: '<message@example.com>',
  subject: 'Project update',
  from: 'Alice <alice@example.com>',
  from_name: 'Alice',
  from_addr: 'alice@example.com',
  to: ['Me <me@example.com>', 'Bob <bob@example.com>'],
  cc: ['Carol <carol@example.com>', 'Alice <alice@example.com>'],
  date: '2026-11-01T10:00:00Z',
  body_text: 'Hello',
  body_html: '<p>Hello</p>',
  body_is_plain_text: false,
  flags: [],
  attachments: [],
  is_suspicious: false,
}

{
  const parsed = parseAddressHeaders(['"Doe, Jane" <jane@example.com>', 'bare@example.com'])
  check('address headers keep quoted names', parsed[0]?.Name === 'Doe, Jane')
  check('address headers parse bare addresses', parsed[1]?.Address === 'bare@example.com')
}

{
  const sender = senderAddress(detail)
  check('sender uses header display name', sender?.Name === 'Alice')
  check('sender uses header address', sender?.Address === 'alice@example.com')
}

{
  const reply = buildReplyEnvelope(detail, 'reply', 'me@example.com')
  check('reply targets only the sender', reply.To.length === 1)
  check('reply leaves Cc empty', reply.Cc.length === 0)
  check('reply sets In-Reply-To', reply.InReplyTo === '<message@example.com>')
  check('reply sets References', reply.References[0] === '<message@example.com>')
}

{
  const replyAll = buildReplyEnvelope(detail, 'reply-all', 'ME@example.com')
  check('reply all keeps the sender in To', replyAll.To[0]?.Address === 'alice@example.com')
  check('reply all excludes the current account', !replyAll.Cc.some((a) => a.Address === 'me@example.com'))
  check('reply all deduplicates the sender from Cc', !replyAll.Cc.some((a) => a.Address === 'alice@example.com'))
  check('reply all keeps other recipients', replyAll.Cc.some((a) => a.Address === 'bob@example.com'))
  check('reply all keeps original Cc', replyAll.Cc.some((a) => a.Address === 'carol@example.com'))
}

{
  const forward = buildReplyEnvelope(detail, 'forward', 'me@example.com')
  check('forward starts with no recipients', forward.To.length === 0 && forward.Cc.length === 0)
  check('forward has no threading headers', !forward.InReplyTo && forward.References.length === 0)
}

{
  check('reply subject gets a prefix', subjectWithPrefix('Hello', 'reply') === 'Re: Hello')
  check('reply subject does not duplicate Re', subjectWithPrefix('Re: Hello', 'reply') === 'Re: Hello')
  check('forward subject gets a prefix', subjectWithPrefix('Hello', 'forward') === 'Fwd: Hello')
  check(
    'forward subject does not duplicate Fwd',
    subjectWithPrefix('Fwd: Hello', 'forward') === 'Fwd: Hello',
  )
}

{
  const escaped = escapeHTML(`<b>"A&B"</b>`)
  check('HTML escaping protects quoted headers', escaped === '&lt;b&gt;&quot;A&amp;B&quot;&lt;/b&gt;')
}

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
