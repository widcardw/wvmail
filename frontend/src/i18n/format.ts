/**
 * The message syntax this app supports — a deliberate subset of vue-i18n's.
 *
 * We do not pull in vue-i18n itself. Two hundred call sites already read a flat,
 * compile-time-checked dictionary (`t['sidebar.unread']`), both locales are bundled
 * statically, and nothing here needs message compilation, lazy loading, or `n()` /
 * `d()` number-and-date formatting. What the guide's *Syntax* and *Pluralization*
 * chapters actually buy us is two features — `{name}` placeholders and
 * `singular | plural` branches — so those are the two implemented here, and only
 * those. The syntax is kept identical to vue-i18n's so the locale files stay
 * portable if the app ever does migrate.
 *
 * What this replaces: hand-rolled `t['mail.remote.blocked'].replace('{count}', …)`
 * at every call site. That version is easy to get wrong (a forgotten `.replace`
 * ships a literal `{count}` to the user), it cannot pluralise, and it cannot
 * escape — `String.replace` treats `$&`, `$1` in the *replacement* as patterns, so
 * a value containing `$` corrupts the output.
 */

/** Values a message placeholder can be filled with. */
export type MessageParams = Record<string, string | number>

const PLACEHOLDER = /\{([\w-]+)\}/g

/**
 * Named interpolation: `{count}` → `params.count`, `{email}` → `params.email`.
 *
 * A placeholder with no matching parameter is left standing rather than blanked,
 * so a typo surfaces in the UI as `{emial}` instead of silently vanishing.
 */
export function formatMessage(template: string, params: MessageParams): string {
  return template.replace(PLACEHOLDER, (whole, name: string) => {
    const value = params[name]
    return value === undefined ? whole : String(value)
  })
}

/**
 * Branch order for languages with more than two plural forms. `Intl.PluralRules`
 * reports the category; a message that lists all of them lists them in this order.
 */
const PLURAL_CATEGORIES = ['zero', 'one', 'two', 'few', 'many', 'other'] as const

// Building an Intl.PluralRules per call would be wasteful — it is called on every
// render of every count-bearing label. There are two locales.
const rulesByLocale = new Map<string, Intl.PluralRules>()

function ruleFor(locale: string): Intl.PluralRules {
  let rule = rulesByLocale.get(locale)
  if (!rule) {
    rule = new Intl.PluralRules(locale)
    rulesByLocale.set(locale, rule)
  }
  return rule
}

function pickBranch(branches: string[], count: number, locale: string): string {
  if (branches.length === 1) return branches[0]
  const category = ruleFor(locale).select(count)
  // `one | other` — the shorthand from the docs, and all English needs.
  if (branches.length === 2) return category === 'one' ? branches[0] : branches[1]
  return branches[PLURAL_CATEGORIES.indexOf(category)] ?? branches[branches.length - 1]
}

/**
 * Pluralisation, vue-i18n style: `'{count} resource | {count} resources'`.
 *
 * `count` selects the branch *and* is available inside it as `{count}` or `{n}`
 * (vue-i18n's two predefined names); pass either in `params` to override the
 * printed value — e.g. to print "one" instead of "1". A language without plural
 * forms — Chinese — writes a single branch, which is then always used.
 *
 * Note this splits on `|`, so a message that is pluralised cannot contain a
 * literal pipe. Non-pluralised messages are never passed through here.
 */
export function formatPlural(
  template: string,
  count: number,
  locale: string,
  params: MessageParams = {},
): string {
  const branches = template.split('|').map((part) => part.trim())
  return formatMessage(pickBranch(branches, count, locale), { count, n: count, ...params })
}
