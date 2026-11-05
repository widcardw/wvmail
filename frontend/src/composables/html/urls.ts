// "Is this URL fetched from off-device?" — the one question both the CSS
// sanitizer and the remote-content blocker need answered.

/** Absolute URL (http/https or protocol-relative), i.e. something fetched off-device. */
export const REMOTE_URL_RE = /^(?:https?:)?\/\//i

export interface SrcSetURLSpan {
  start: number
  end: number
  url: string
}

/**
 * Locate the URL token in each `srcset` candidate. A normal URL ends at a comma
 * even when the next candidate has no leading space. A data URI keeps consuming
 * commas until whitespace, because the comma after `data:image/…;base64` is part
 * of the payload; splitting there would mistake its `//` for a remote URL.
 */
export function srcSetURLSpans(value: string): SrcSetURLSpan[] {
  const spans: SrcSetURLSpan[] = []
  let i = 0
  while (i < value.length) {
    while (i < value.length && (value[i] === ',' || /\s/.test(value[i]))) i++
    if (i >= value.length) break

    const start = i
    const isDataURI = value.slice(start, start + 5).toLowerCase() === 'data:'
    while (i < value.length && !/\s/.test(value[i])) {
      // A normal srcset URL ends at a comma even when the next candidate has no
      // leading space. A data URI's first comma is part of the payload, so keep
      // consuming it until whitespace.
      if (!isDataURI && value[i] === ',') break
      i++
    }
    let end = i
    if (end > start && value[end - 1] === ',') {
      end--
      spans.push({ start, end, url: value.slice(start, end) })
      continue
    }

    spans.push({ start, end, url: value.slice(start, end) })
    // Skip the descriptor ("1x", "640w", …) up to the next candidate.
    while (i < value.length && value[i] !== ',') i++
    if (i < value.length) i++
  }
  return spans
}

/**
 * Attributes that make a browser fetch a resource. Both spellings of `srcset`
 * are listed on purpose: hast stores property names camel-cased (`srcSet`), and
 * a missed spelling would leave `<picture><source srcset=…>` able to load a
 * remote tracking image.
 */

/** True when any candidate URL inside the attribute value points off-device. */
export function hasRemoteRef(value: string): boolean {
  return srcSetURLSpans(value).some(({ url }) => REMOTE_URL_RE.test(url))
}

/**
 * Sanitize one CSS text (a `<style>` block or a `style=` attribute value).
 *
 * This is a pragmatic blocklist, not a full CSS parser: it removes the
 * constructs that would let message CSS fetch remote resources or escape the
 * message area, while leaving layout/colour declarations untouched so email
 * still renders. Remote `url()` are only stripped when remote content is
 * blocked; `data:`/`cid:` (our resolved inline images) are always kept.
 */
/**
 * Longest single declaration we keep. Real mail never ships a value anywhere near this;
 * spam does — a 127 KB `box-shadow` list is one pixel per entry, which is how a message
 * draws a picture without an `<img>`, a URL or an attachment, defeating image blocking
 * (and URL filters) by construction. `see the spam from cy@zzbh.org` for the real one.
 */
