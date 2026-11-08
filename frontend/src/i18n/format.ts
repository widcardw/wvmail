/** Format standalone strings with named placeholders; translations use vue-i18n. */

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
