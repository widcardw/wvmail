export interface FolderTreeMailbox {
  name: string
  delim: string
  attrs: string[]
}

export interface FolderTreeNode {
  /** Full IMAP mailbox name used for selection and fetching. */
  fullName: string
  /** Display label for this segment only. */
  label: string
  icon: string
  rank: number
  selectable: boolean
  unread?: number
  children: FolderTreeNode[]
}

export interface FolderTreeDisplay {
  label: string
  icon: string
  rank: number
}

interface MutableFolderTreeNode extends FolderTreeNode {
  childMap: Map<string, MutableFolderTreeNode>
}

/**
 * Rebuild an IMAP folder list into a hierarchy using each mailbox's delimiter.
 * Non-selectable IMAP parents are retained when they own visible descendants;
 * missing intermediate parents are synthesized from child mailbox names.
 */
export function buildFolderTree(
  folders: FolderTreeMailbox[],
  displaySegment: (segment: string) => FolderTreeDisplay = (segment) => ({
    label: segment.replace(/^\[.*?\]\//, ''),
    icon: 'ri:folder-2-line',
    rank: 99,
  }),
): FolderTreeNode[] {
  const roots = new Map<string, MutableFolderTreeNode>()

  for (const folder of folders) {
    const attrs = folder.attrs.map((attr) => attr.toLowerCase())
    if (attrs.some((attr) => attr.includes('nonexistent'))) continue

    const delimiter = folder.delim
    // Gmail's special namespace is a provider grouping, not a user mailbox
    // parent. Keep its system folders at the top level as before.
    const isGmailSystemFolder = folder.name.toLowerCase().startsWith('[gmail]/')
    const parts = delimiter && !isGmailSystemFolder ? folder.name.split(delimiter) : [folder.name]
    if (parts.some((part) => part === '')) continue

    let siblings = roots
    let fullName = ''
    let current: MutableFolderTreeNode | undefined
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i]
      fullName = i === 0 ? part : `${fullName}${delimiter}${part}`
      current = siblings.get(part)
      if (!current) {
        const display = displaySegment(part)
        current = {
          fullName,
          label: display.label,
          icon: display.icon,
          rank: display.rank,
          selectable: false,
          children: [],
          childMap: new Map(),
        }
        siblings.set(part, current)
      }
      if (i === parts.length - 1) {
        current.selectable = !attrs.some((attr) => attr.includes('noselect'))
      }
      siblings = current.childMap
    }
  }

  const materialize = (nodes: Map<string, MutableFolderTreeNode>): FolderTreeNode[] => {
    return [...nodes.values()]
      .filter((node) => node.selectable || node.childMap.size > 0)
      .sort((a, b) => a.rank - b.rank || a.label.localeCompare(b.label))
      .map(({ childMap, ...node }) => ({
        ...node,
        children: materialize(childMap),
      }))
  }

  return materialize(roots)
}
