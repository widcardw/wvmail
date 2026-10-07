// "Is this URL fetched from off-device?" — the one question both the CSS
// sanitizer and the remote-content blocker need answered.

/** Absolute URL (http/https or protocol-relative), i.e. something fetched off-device. */
export const REMOTE_URL_RE = /^(?:https?:)?\/\//i

/**
 * Attributes that make a browser fetch a resource. Both spellings of `srcset`
 * are listed on purpose: hast stores property names camel-cased (`srcSet`), and
 * a missed spelling would leave `<picture><source srcset=…>` able to load a
 * remote tracking image.
 */

/** True when any candidate URL inside the attribute value points off-device. */
export function hasRemoteRef(value: string): boolean {
  // srcset is a comma-separated list of "url descriptor" candidates.
  return value
    .split(',')
    .some((candidate) => REMOTE_URL_RE.test(candidate.trim().split(/\s+/)[0] ?? ''))
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
