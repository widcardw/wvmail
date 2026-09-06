import { h, type VNode } from 'vue'

export interface IconSource {
  type: 'iconify' | 'image' | 'component'
  create: () => VNode
}

/**
 * Provider account glyphs rendered offline (no lib icon runtime).
 * - brand "logos:*" icons: use the offline `i-logos:...` UnoCSS classes (safelisted).
 * - 163 / qq: hand-built colored chip / local PNG.
 */

function brandCreate(iconName: string, size?: number): () => VNode {
  return () =>
    h('i', {
      class: `i-${iconName}`,
      style: {
        fontSize: size ? `${size}px` : undefined,
        verticalAlign: 'middle',
      },
    })
}

export const ICON_MAPPING: Record<string, IconSource> = {
  gmail: {
    type: 'iconify',
    create: brandCreate('logos:google-icon', 16),
  },
  outlook: {
    type: 'iconify',
    create: brandCreate('logos:microsoft-windows-icon', 16),
  },
  '163': {
    type: 'component',
    create: () =>
      h(
        'span',
        {
          style: {
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: '18px',
            height: '18px',
            borderRadius: '5px',
            background: '#FF3333',
            color: '#fff',
            fontSize: '9px',
            fontWeight: 700,
            userSelect: 'none',
          },
        },
        '163',
      ),
  },
  qq: {
    type: 'component',
    create: () => h('img', { src: '/qq_macos_bigsur_icon.png', width: 18, height: 18, alt: 'QQ' }),
  },
}

export const ICON_MAPPING_SMALL: Record<string, IconSource> = {
  gmail: {
    type: 'iconify',
    create: brandCreate('logos:google-icon', 16),
  },
  outlook: {
    type: 'iconify',
    create: brandCreate('logos:microsoft-windows-icon', 16),
  },
  '163': {
    type: 'component',
    create: () =>
      h(
        'span',
        {
          style: {
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: '16px',
            height: '16px',
            borderRadius: '4px',
            background: '#FF3333',
            color: '#fff',
            fontSize: '6px',
            fontWeight: 700,
            userSelect: 'none',
          },
        },
        '163',
      ),
  },
  qq: {
    type: 'component',
    create: () => h('img', { src: '/qq_macos_bigsur_icon.png', width: 16, height: 16, alt: 'QQ' }),
  },
}

export { brandCreate }
