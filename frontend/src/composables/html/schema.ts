import { defaultSchema } from 'rehype-sanitize'
import type { Schema } from 'hast-util-sanitize'

// The rehype-sanitize schema for mail: which tags/attributes/protocols survive.
// Remote `src` is allowed here on purpose — blocking it is a separate pass that can
// count and label what it removed (see remote.ts).

type PropertyDefinition = [string, ...(string | number | boolean | RegExp | null | undefined)[]]

export const emailSchema: Schema = {
  ...defaultSchema,
  tagNames: [...(defaultSchema.tagNames ?? []), 'style'],
  attributes: {
    ...defaultSchema.attributes,
    '*': [...(defaultSchema.attributes?.['*'] ?? []), ['style'] as PropertyDefinition],
    a: [
      ...(defaultSchema.attributes?.a ?? []),
      ['className', 'email-auto-link'] as PropertyDefinition,
    ],
  },
  protocols: {
    ...defaultSchema.protocols,
    src: [
      ...((defaultSchema.protocols as Record<string, string[]> | undefined)?.['src'] ?? []),
      'data',
      'cid',
    ],
  },
}

/**
 * Placeholder that replaces a blocked `<img>`.
 *
 * The element is swapped rather than restyled because an `<img>` is a replaced
 * element: WebKit does not render its pseudo-elements, so alt text cannot be
 * styled or overlaid on it. A `<span>` gives us the striped background, the
 * inline icon and the alt text in one box.
 */
