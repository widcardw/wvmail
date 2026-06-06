import { h } from 'vue'
import type { VNode } from 'vue'
import AppIcon from '../components/AppIcon.vue'
import type { AppIconSource } from '../components/AppIcon.vue'

/**
 * Composable providing a `renderIcon()` factory for creating AppIcon VNodes
 * programmatically — useful for Naive UI render functions (NMenu options,
 * NButton icon slots, NDataTable columns, etc.).
 */
export function useIconRenderer() {
  /**
   * Returns a `() => VNode` factory suitable for Naive UI's `icon` render prop.
   *
   * @example
   * ```ts
   * const { renderIcon } = useIconRenderer()
   * const menuOptions = [
   *   { label: 'Home', key: 'home', icon: renderIcon('ri:home-3-line') },
   *   { label: 'OpenAI', key: 'openai', icon: renderIcon('openai', 'lobehub', 20) },
   * ]
   * ```
   */
  function renderIcon(icon: string, source?: AppIconSource, size?: number): () => VNode {
    return () => h(AppIcon, { icon, source, size })
  }

  return { renderIcon }
}
