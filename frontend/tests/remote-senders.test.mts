/**
 * Allowed remote-content sender matching.
 *
 * Supported forms:
 *  - `user@example.com` — exact address
 *  - `@example.com` — every address on that domain
 *  - `news@*.example.com` — one wildcard label per dot-separated label
 *
 * Wildcards do not cross domain labels or the local/domain boundary.
 */
import {
  appendRemoteSenderPattern,
  isValidRemoteSenderPattern,
  replaceRemoteSenderPattern,
  senderMatchesPattern,
} from '../src/composables/remote-senders.ts'

let pass = 0
let fail = 0

function check(name: string, cond: boolean) {
  if (cond) {
    pass++
    console.log(`  ok  ${name}`)
  } else {
    fail++
    console.error(`  FAIL  ${name}`)
  }
}

console.log('pattern validation')
check('an address is valid', isValidRemoteSenderPattern(' User@Example.com '))
check('a whole-domain entry is valid', isValidRemoteSenderPattern('@example.com'))
check('a local wildcard is valid', isValidRemoteSenderPattern('news@*.example.com'))
check('a domain wildcard is valid', isValidRemoteSenderPattern('user@example.*'))
check('a plus-addressed sender is valid', isValidRemoteSenderPattern('user+tag@example.com'))
check(
  'a regex metacharacter in the local part is valid',
  isValidRemoteSenderPattern('cost$@example.com'),
)
check('an empty entry is rejected', !isValidRemoteSenderPattern('   '))
check('a bare word is rejected', !isValidRemoteSenderPattern('example.com'))
check('two @ signs are rejected', !isValidRemoteSenderPattern('a@b@example.com'))
check('an empty wildcard label is rejected', !isValidRemoteSenderPattern('news@..example.com'))

console.log('matching')
check('exact address matches', senderMatchesPattern('user@example.com', 'USER@EXAMPLE.COM'))
check(
  'exact address rejects another address',
  !senderMatchesPattern('other@example.com', 'user@example.com'),
)
check(
  'a plus sign is matched literally',
  senderMatchesPattern('user+tag@example.com', 'user+tag@example.com'),
)
check(
  'a plus sign is not regex syntax',
  !senderMatchesPattern('usertag@example.com', 'user+tag@example.com'),
)
check(
  'a dollar sign is matched literally',
  senderMatchesPattern('cost$@example.com', 'cost$@example.com'),
)
check(
  'whole-domain entry matches subdomain sender',
  senderMatchesPattern('news@mail.example.com', '@example.com'),
)
check(
  'whole-domain entry matches its own domain',
  senderMatchesPattern('news@example.com', '@example.com'),
)
check(
  'whole-domain entry rejects another domain',
  !senderMatchesPattern('user@example.org', '@example.com'),
)
check(
  'local wildcard matches one label',
  senderMatchesPattern('news@sub.example.com', 'news@*.example.com'),
)
check(
  'local wildcard is label-bound',
  !senderMatchesPattern('news@a.b.example.com', 'news@*.example.com'),
)
check(
  'domain wildcard matches one suffix label',
  senderMatchesPattern('user@example.com', 'user@example.*'),
)
check(
  'domain wildcard is label-bound',
  !senderMatchesPattern('user@example.co.uk', 'user@example.*'),
)
check(
  'domain wildcard does not add a subdomain',
  !senderMatchesPattern('user@sub.example.com', 'user@example.*'),
)
check(
  'local and domain wildcards combine',
  senderMatchesPattern('x@mail.example.com', '*@*.example.com'),
)
check(
  'a local wildcard matches any local part',
  senderMatchesPattern('user@example.com', '*@example.com'),
)
check(
  'a local wildcard includes plus addressing',
  senderMatchesPattern('user+tag@example.com', '*@example.com'),
)
check('an invalid pattern never matches', !senderMatchesPattern('user@example.com', 'example.com'))

console.log('list changes')
const initial = ['user@example.com']
const appended = appendRemoteSenderPattern(initial, ' News@*.Example.com ')
check(
  'adding a pattern normalizes and appends it',
  appended.status === 'added' &&
    appended.senders.join(',') === 'user@example.com,news@*.example.com',
)
check(
  'an exact duplicate is rejected',
  appendRemoteSenderPattern(initial, 'USER@example.com').status === 'duplicate',
)
check(
  'an address covered by a wildcard can still be added',
  appendRemoteSenderPattern(['*@example.com'], 'user@example.com').status === 'added',
)
check(
  'an invalid addition is rejected',
  appendRemoteSenderPattern(initial, 'example.com').status === 'invalid',
)

const replaced = replaceRemoteSenderPattern(initial, 'user@example.com', '@example.com')
check(
  'editing replaces the old pattern',
  replaced.status === 'updated' && replaced.senders.join(',') === '@example.com',
)
check(
  'editing to an existing pattern is rejected',
  replaceRemoteSenderPattern(
    ['user@example.com', '@example.com'],
    'user@example.com',
    '@example.com',
  ).status === 'duplicate',
)
check(
  'editing to an invalid pattern is rejected',
  replaceRemoteSenderPattern(initial, 'user@example.com', 'not-an-address').status === 'invalid',
)

console.log(`\n${pass} passed, ${fail} failed`)
if (fail > 0) process.exit(1)
