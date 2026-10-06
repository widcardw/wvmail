// Provider avatars from @lobehub/icons-static-avatar (WebP)
// These have embedded backgrounds so they never blend into dark/light themes.
import deepseekIcon from '@lobehub/icons-static-avatar/avatars/deepseek.webp'
import openaiIcon from '@lobehub/icons-static-avatar/avatars/openai.webp'
import anthropicIcon from '@lobehub/icons-static-avatar/avatars/anthropic.webp'
import geminiIcon from '@lobehub/icons-static-avatar/avatars/gemini.webp'
import minimaxIcon from '@lobehub/icons-static-avatar/avatars/minimax.webp'
import kimiIcon from '@lobehub/icons-static-avatar/avatars/kimi.webp'
import ollamaIcon from '@lobehub/icons-static-avatar/avatars/ollama.webp'
import volcIcon from '@lobehub/icons-static-avatar/avatars/volcengine.webp'
import { h } from 'vue'
import type { VNode } from 'vue'

const ICON_MAP: Record<string, string> = {
  deepseek: deepseekIcon,
  openai: openaiIcon,
  anthropic: anthropicIcon,
  gemini: geminiIcon,
  minimax: minimaxIcon,
  kimi: kimiIcon,
  ollama: ollamaIcon,
  volcengine: volcIcon,
}

/** Iconify icons for providers without a lobehub avatar */
const FALLBACK_ICON_MAP: Record<string, string> = {
  custom: 'streamline-color:brain',
}

/**
 * Returns the avatar URL for a given provider, or undefined if no icon exists.
 */
export function getProviderIconUrl(provider: string): string | undefined {
  return ICON_MAP[provider]
}

/**
 * Returns the icon configuration (source type + icon identifier) for a given
 * provider, or null if no icon exists. Handles both lobehub avatars and
 * iconify fallbacks (e.g. `custom`).
 */
export function getProviderIconConfig(
  provider: string,
): { source: 'lobehub' | 'iconify'; icon: string } | null {
  if (ICON_MAP[provider]) {
    return { source: 'lobehub', icon: provider }
  }
  if (FALLBACK_ICON_MAP[provider]) {
    return { source: 'iconify', icon: FALLBACK_ICON_MAP[provider] }
  }
  return null
}

/**
 * Renders a provider avatar as a VNode (useful for NSelect render-label,
 * NTag slots, etc.).
 */
export function renderProviderIcon(provider: string, size = 16): VNode | null {
  const src = ICON_MAP[provider]
  if (src) {
    return h('img', {
      src,
      width: size,
      height: size,
      style: {
        verticalAlign: 'middle',
        display: 'inline-block',
        borderRadius: '4px',
      },
    })
  }
  const fallbackIcon = FALLBACK_ICON_MAP[provider]
  if (fallbackIcon) {
    // offline iconify fallback via UnoCSS class (see uno.config `safelist`)
    return h('i', { class: ` i-${fallbackIcon}`, style: { fontSize: `${size}px` } })
  }
  return null
}

/**
 * Renders a provider avatar + label pair for use in select options / tags / rows.
 */
export function renderProviderOption(provider: string, label: string, size = 16): VNode {
  const icon = renderProviderIcon(provider, size)
  return h(
    'span',
    {
      style: {
        display: 'inline-flex',
        alignItems: 'center',
        gap: '0.25rem',
      },
    },
    [
      icon,
      h(
        'span',
        {
          style: {
            marginLeft: icon ? '6px' : '0',
            verticalAlign: 'middle',
            borderRadius: '0.25rem',
          },
        },
        label,
      ),
    ],
  )
}
