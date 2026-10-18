// Always-on layout rules for the email body: keep oversized content from
// breaking the reading pane. Images scale down to the pane width instead of
// overflowing; tables sit in a horizontally-scrollable wrapper so wide data
// tables can scroll. Frozen `white-space: pre` / `nowrap` lines (which can also
// push past the edge) are handled in css.ts — it rewrites only the declaration
// itself, so normal body layout is left untouched.
//
// These styles live inside the body shadow root, so plain `img` /
// `.email-table-scroll` selectors are already scoped — no parent class needed.

export const CONTENT_FIT_CSS = `
img {
  max-width: 100% !important;
  height: auto !important;
  box-sizing: border-box;
}
.email-table-scroll {
  display: block;
  max-width: 100%;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
}
/* No forced width on the wrapped table: a layout table (many transactional
   mails, e.g. Vercel, build the whole message in a width="100%" table) must
   keep filling the pane and wrapping its text. Only a table the sender sized
   wider than the pane overflows, and then this wrapper scrolls it. Forcing the
   table to its intrinsic shrink-to-fit width here would break that auto-wrap
   and drag the table into a single over-wide row — the opposite of what we want. */
.email-table-scroll > table {
  width: auto;
}
/* The <pre> tag gets its white-space from the UA stylesheet (not the sender),
   so css.ts cannot see it. Demote it to pre-wrap with break points so a <pre>
   code block can still wrap on a narrow pane. This targets the tag only — it
   does not touch any element the sender laid out normally, so it cannot inflate
   the message with blank rows the way a global white-space rule would. */
pre {
  white-space: pre-wrap !important;
  overflow-wrap: anywhere !important;
  word-break: break-word !important;
}
`
