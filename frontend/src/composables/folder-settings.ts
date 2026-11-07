import type { FolderTreeNode } from './folder-tree'

/**
 * The account-owned subset of a folder display rule. Keeping this structural
 * (instead of importing the generated Wails class) lets the folder tree and its
 * tests stay independent of the binding runtime.
 */
export interface AccountFolderSettingLike {
  Name: string
  Hidden?: boolean
  DisplayName?: string
  Icon?: string
}

export interface FolderDisplayOverride {
  hidden: boolean
  label?: string
  icon?: string
}

/** Find one mailbox rule by its full IMAP name. */
export function folderSettingFor(
  settings: readonly AccountFolderSettingLike[] | undefined,
  folderName: string,
): AccountFolderSettingLike | undefined {
  return settings?.find((setting) => setting.Name === folderName)
}

/**
 * Resolve the effective override. Empty/whitespace strings mean "keep the
 * automatic value", which is also what an omitted rule means.
 */
export function folderDisplayOverride(
  settings: readonly AccountFolderSettingLike[] | undefined,
  folderName: string,
): FolderDisplayOverride {
  const setting = folderSettingFor(settings, folderName)
  return {
    hidden: setting?.Hidden === true,
    label: setting?.DisplayName?.trim() || undefined,
    icon: setting?.Icon?.trim() || undefined,
  }
}

/**
 * Reorder a folder tree from one flat list of mailbox names.
 *
 * `FolderOrder` is saved as a flat list because the settings page renders a
 * single list, but ordering only means something among siblings: a child can
 * never leave its parent. Each level therefore sorts by the mailbox's position
 * in that list, and names that are not listed keep their automatic (rank) order
 * after the listed ones. Sorting is stable, so equal siblings stay put.
 */
export function orderFolderNodes(
  nodes: readonly FolderTreeNode[],
  order?: readonly string[],
): FolderTreeNode[] {
  if (!order?.length) return [...nodes]

  const position = new Map(order.map((name, index) => [name, index]))
  const ordered = [...nodes].sort((a, b) => {
    const ai = position.get(a.fullName)
    const bi = position.get(b.fullName)
    if (ai === undefined && bi === undefined) return 0
    if (ai === undefined) return 1
    if (bi === undefined) return -1
    return ai - bi
  })

  return ordered.map((node) => ({
    ...node,
    children: orderFolderNodes(node.children, order),
  }))
}

/**
 * Apply per-mailbox rules to a finished folder tree.
 *
 * A hidden folder hides its descendants too: a child whose parent is not in the
 * sidebar would otherwise appear detached from its actual mailbox path. A
 * non-selectable parent is dropped when all of its children were hidden.
 */
export function applyFolderSettings(
  folders: readonly FolderTreeNode[],
  settings: readonly AccountFolderSettingLike[] | undefined,
  order?: readonly string[],
): FolderTreeNode[] {
  if (!settings?.length && !order?.length) return [...folders]

  const byName = new Map((settings ?? []).map((setting) => [setting.Name, setting]))

  function apply(node: FolderTreeNode): FolderTreeNode | null {
    const setting = byName.get(node.fullName)
    if (setting?.Hidden === true) return null

    const children = node.children
      .map(apply)
      .filter((child): child is FolderTreeNode => child !== null)
    if (!node.selectable && children.length === 0) return null

    return {
      ...node,
      label: setting?.DisplayName?.trim() || node.label,
      icon: setting?.Icon?.trim() || node.icon,
      children,
    }
  }

  const configured = folders.map(apply).filter((node): node is FolderTreeNode => node !== null)
  return orderFolderNodes(configured, order)
}
