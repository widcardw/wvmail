/** Basic HTML sanitizer for email body rendering.
 *  Strips dangerous tags and event handler attributes. */
function sanitizeHtml(html: string): string {
  // Remove dangerous tags
  const dangerous =
    /<\s*\b(script|iframe|object|embed|frame|form|input|style)\b[^>]*>.*?<\/\s*\1\s*>|<(script|iframe|object|embed|frame|form|input|style)\b[^>]*\/?\s*>/gis
  let clean = html.replace(dangerous, '')
  // Remove on* event handlers
  clean = clean.replace(/\s+on\w+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, '')
  // Remove javascript: and data: URIs in href/src
  clean = clean.replace(
    /(?:href|src|action)\s*=\s*(?:"javascript:[^"]*"|'javascript:[^']*'|javascript:[^\s>]+)/gi,
    '',
  )
  clean = clean.replace(
    /(?:href|src|action)\s*=\s*(?:"data:[^"]*"|'data:[^']*'|data:[^\s>]+)/gi,
    '',
  )
  return clean
}
export { sanitizeHtml }
