import { h, type VNode } from 'vue'

export interface IconSource {
  type: 'iconify' | 'image' | 'component'
  create: (size: number) => VNode
}

/**
 * Provider account glyphs rendered offline (no lib icon runtime).
 * - brand "logos:*" icons: use the offline `i-logos:...` UnoCSS classes (safelisted).
 * - 163 / qq: hand-built colored chip / local PNG.
 */

function brandCreate(iconName: string): (size: number) => VNode {
  return (size) =>
    h('i', {
      class: `i-${iconName}`,
      style: {
        display: 'block',
        width: `${size}px`,
        height: `${size}px`,
        fontSize: `${size}px`,
        flexShrink: 0,
      },
    })
}

export const ICON_MAPPING: Record<string, IconSource> = {
  gmail: {
    type: 'iconify',
    create: brandCreate('logos:google-icon'),
  },
  outlook: {
    type: 'iconify',
    create: brandCreate('logos:microsoft-windows-icon'),
  },
  '163': {
    type: 'component',
    create: (size) =>
      h(
        'span',
        {
          style: {
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: `${size}px`,
            height: `${size}px`,
            borderRadius: `${Math.max(3, Math.round(size * 0.24))}px`,
            background: '#FF3333',
            color: '#fff',
            fontSize: `${Math.max(7, Math.round(size * 0.36))}px`,
            fontWeight: 700,
            userSelect: 'none',
          },
        },
        '163',
      ),
  },
  qq: {
    type: 'component',
    create: (size) =>
      h('img', {
        src: '/qq_macos_bigsur_icon.png',
        alt: 'QQ',
        style: {
          display: 'block',
          width: `${size}px`,
          height: `${size}px`,
          objectFit: 'cover',
        },
      }),
  },
}

export { brandCreate }
