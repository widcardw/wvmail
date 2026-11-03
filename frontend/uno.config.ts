import { defineConfig, presetUno, presetIcons, transformerDirectives } from 'unocss'

// UnoCSS config (wvmail)
// - presetIcons: build-time, offline icons from installed `@iconify-json/*` sets.
//   Usage: class="i-ri:sun-line". Literal class strings are picked up from scanned
//   sources (.vue templates + maps) so icons are bundled with no runtime/CDN fetch.
// - presetUno (wind) gives the utility tokens; fine-grained palette stays in
//   src/styles/global.css unless we map them here later.
//
// The light/dark design tokens live in src/styles/global.css under :root and
// [data-theme='dark']; anything that flips (borders, surface, text) should use those
// vars rather than be hardtyped here. The same applies to fonts: `font-sans` and
// `font-mono` are mapped onto `--font-sans` / `--font-mono` below, so the utilities
// and the CSS declarations can never drift apart.
export default defineConfig({
  theme: {
    fontFamily: {
      sans: 'var(--font-sans)',
      mono: 'var(--font-mono)',
    },
  },
  presets: [
    presetUno(),
    presetIcons({
      scale: 1,
      // Icons need no dynamic collection config: preset resolves "i-<set>:<name>"
      // from any installed @iconify-json/<set>. Keep warn off by default.
      extraProperties: {
        display: 'inline-block',
        'vertical-align': 'middle',
      },
    }),
  ],
  transformers: [transformerDirectives()],
  // Iconify names that only exist as runtime strings (map-driven icons, attachment
  // types, provider fallbacks, avatars) must still be emitted offline, so list them
  // explicitly. These correspond to `@iconify-json/*` collections added as deps.
  safelist: [
    // shell / chrome
    'i-ri:sun-line',
    'i-ri:moon-clear-line',
    'i-ri:computer-line',
    'i-ri:inbox-archive-line',
    'i-ri:notification-2-line',
    'i-ri:contacts-book-2-line',
    'i-ri:pen-nib-line',
    'i-ri:pen-nib-fill',
    // mail folder categories
    'i-ri:inbox-2-line',
    'i-ri:inbox-2-line',
    'i-ri:send-plane-line',
    'i-ri:send-plane-2-line',
    'i-ri:draft-line',
    'i-ri:delete-bin-5-line',
    'i-ri:spam-2-line',
    'i-ri:archive-2-line',
    'i-ri:star-line',
    'i-ri:star-fill',
    'i-ri:notification-3-line',
    'i-ri:folder-2-line',
    'i-ri:draft-line',
    'i-ri:home-2-line',
    'i-ri:settings-2-line',
    // misc actions/glyphs passed to <AppIcon>
    'i-ri:account-circle-2-line',
    'i-ri:close-line',
    'i-ri:mail-line',
    'i-ri:mail-unread-line',
    'i-ri:reply-line',
    'i-ri:reply-all-line',
    'i-ri:share-forward-line',
    'i-ri:translate-2',
    'i-ri:information-line',
    'i-ri:shield-check-line',
    'i-ri:github-fill',
    'i-ri:external-link-line',
    // Settings → Proxy: the nav icon is built as `i-${item.icon}`, and the
    // proxy test's result icons come from a ternary — neither is visible to the
    // scanner.
    'i-ri:route-line',
    'i-ri:checkbox-circle-line',
    'i-ri:error-warning-line',
    'i-ri:refresh-line',
    'i-ri:download-2-line',
    'i-ri:arrow-go-back-line',
    'i-ri:arrow-go-forward-line',
    'i-ri:robot-2-line',
    // provider fallback (LLM/avatar) + brand logos + attachments
    'i-streamline-color:brain',
    'i-logos:google-icon',
    'i-logos:microsoft-windows-icon',
    'i-logos:typescript-icon',
    'i-logos:javascript',
    'i-logos:python',
    'i-logos:java',
    'i-vscode-icons:file-type-image',
    'i-vscode-icons:file-type-video',
    'i-vscode-icons:file-type-audio',
    'i-vscode-icons:file-type-pdf2',
    'i-vscode-icons:file-type-zip',
    'i-vscode-icons:file-type-word',
    'i-vscode-icons:file-type-excel',
    'i-vscode-icons:file-type-powerpoint',
    'i-vscode-icons:file-type-text',
    'i-vscode-icons:file-type-json',
    'i-vscode-icons:file-type-xml',
    'i-vscode-icons:file-type-markdown',
    'i-vscode-icons:file-type-light-yaml',
    'i-vscode-icons:default-file',
    'i-ri:arrow-right-s-line',
    'i-ri:virus-line',
    'i-ri:sparkling-line',
    // Sidebar connection markers: the class names come from markerIcon() in
    // SideBar.vue, so the scanner cannot see them.
    'i-streamline:broken-link-2',
    'i-streamline:interface-time-midnight-whole-midnight-hour-clock-time',
    'i-mdi:snail',
    'i-mdi:chevron-right',
    'i-streamline:interface-link-broken-break-broken-hyperlink-link-remove-unlink',
  ],
  shortcuts: {
    // 'chip' etc. reserved if we need one-token util aliases.
  },
})
