// Always-on layout rules for the email body: keep oversized content from
// breaking the reading pane. Images scale down to the pane width instead of
// overflowing; tables sit in a horizontally-scrollable wrapper so columns
// stay readable. The body's `word-break: break-word` (EmailShadowBody) handles
// long words/URLs.
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
/* Tables inside keep their declared width so the wrapper — not the
   table — scrolls, which preserves column widths the sender chose. */
.email-table-scroll > table {
  width: max-content;
}
`
