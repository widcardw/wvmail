import { h, VNode } from 'vue'
import { NIcon, NIconWrapper } from 'naive-ui'
import { Icon } from '@iconify/vue'
export interface IconSource {
  type: 'iconify' | 'image' | 'component'
  create: () => VNode
}

export const ICON_MAPPING: Record<string, IconSource> = {
  gmail: {
    type: 'iconify',
    create: () => h(NIcon, null, { default: () => h(Icon, { icon: 'logos:google-icon' }) }),
  },
  outlook: {
    type: 'iconify',
    create: () =>
      h(NIcon, null, { default: () => h(Icon, { icon: 'logos:microsoft-windows-icon' }) }),
  },
  '163': {
    type: 'component',
    create: () =>
      h(
        NIconWrapper,
        { borderRadius: 5, color: '#FF3333', iconColor: 'white' },
        {
          default: () =>
            h(NIcon, null, {
              default: () => h('div', { style: { fontSize: '9px' } }, { default: () => '163' }),
            }),
        },
      ),
  },
  qq: {
    type: 'component',
    create: () =>
      h(
        NIconWrapper,
        {color: '#00000000'},
        {
          default: () => h('img', {src: '/qq_macos_bigsur_icon.png', width: 30, height: 30})
        }
      )
      // h(
      //   NIconWrapper,
      //   { borderRadius: 5, color: '#EF790D', iconColor: 'white' },
      //   {
      //     default: () =>
      //       h(
      //         NIcon,
      //         { size: '20' },
      //         {
      //           default: () => h(Icon, { icon: 'ri:mail-line' }),
      //         },
      //       ),
      //   },
      // ),
  },
}

export const ICON_MAPPING_SMALL: Record<string, IconSource> = {
  gmail: {
    type: 'iconify',
    create: () => h(NIcon, {size: 16}, { default: () => h(Icon, { icon: 'logos:google-icon' }) }),
  },
  outlook: {
    type: 'iconify',
    create: () =>
      h(NIcon, {size: 16}, { default: () => h(Icon, { icon: 'logos:microsoft-windows-icon' }) }),
  },
  '163': {
    type: 'component',
    create: () =>
      h(
        NIconWrapper,
        { borderRadius: 3, color: '#FF3333', iconColor: 'white', size: 16 },
        {
          default: () =>
            h(NIcon, {size: 16}, {
              default: () => h('div', { style: { fontSize: '6px' } }, { default: () => '163' }),
            }),
        },
      ),
  },
  qq: {
    type: 'component',
    create: () =>
      h(
        NIcon,
        {size: 16},
        {
          default: () => h('img', {src: '/qq_macos_bigsur_icon.png', width: 16, height: 16})
        }
      )
      // h(
      //   NIconWrapper,
      //   { borderRadius: 5, color: '#EF790D', iconColor: 'white' },
      //   {
      //     default: () =>
      //       h(
      //         NIcon,
      //         { size: '20' },
      //         {
      //           default: () => h(Icon, { icon: 'ri:mail-line' }),
      //         },
      //       ),
      //   },
      // ),
  },
}
