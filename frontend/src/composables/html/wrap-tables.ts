import type { Root, Element, Parent } from 'hast'

// Wrap each top-level (and nested, non-already-wrapped) <table> in a
// <div class="email-table-scroll"> so a wide table can grow horizontally
// without breaking the page flow. Sanitising runs first and keeps every
// tag we act on, so the tree here is already safe to mutate.
//
// In `fragment: true` mode rehype-parse puts top-level nodes directly under
// the `Root`, so a stray table that arrived bare (a mail with no <body>)
// must be wrapped at Root level — Root.children is still the editable list.
//
// We walk top-down and pass the parent in explicitly. Reading `node.parent`
// would have been easier, but unified only back-fills that field after the
// whole pipeline runs — too late for a plugin that mutates the tree.
//
// After wrapping a table we keep walking its *original* subtree (not the new
// wrapper) so nested tables also get scroll containers — but we never visit
// the same table twice because the walk path never revisits a node.

const SCROLL_CLASS = 'email-table-scroll'

function wrap(table: Element, parent: Parent): Element | null {
  const children = (parent as { children: typeof parent.children }).children
  const idx = children.indexOf(table)
  if (idx < 0) return null
  const wrapper: Element = {
    type: 'element',
    tagName: 'div',
    properties: { className: [SCROLL_CLASS] },
    children: [table],
  }
  children.splice(idx, 1, wrapper)
  return wrapper
}

function walk(node: Parent, visit: (el: Element, parent: Parent) => void): void {
  if (!('children' in node)) return
  const children = node.children
  for (let i = 0; i < children.length; i++) {
    const child = children[i]
    if (child.type !== 'element') continue
    if (child.tagName === 'table') {
      visit(child, node)
      // children[i] is now { children: [table] } — the wrapper. Continue
      // into the table's own subtree so nested tables are wrapped too.
      walk(child, visit)
    } else {
      walk(child, visit)
    }
  }
}

/** Wrap every <table> in a horizontally-scrollable div. */
export function rehypeWrapTables() {
  return (tree: Root) => {
    walk(tree, (table, parent) => wrap(table, parent))
  }
}
