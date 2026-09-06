import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import UnoCSS from 'unocss/vite'
import wails from '@wailsio/runtime/plugins/vite'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue(), UnoCSS(), wails('./bindings')],
  server: {
    // wails3 backend failed to dial 127.0.0.1:9245,
    // since by default the server starts at `localhost` instead of `127.0.0.1`
    host: '127.0.0.1',
  },
  resolve: {
    alias: {
      '~': path.resolve(__dirname, 'src'),
      '#': path.resolve(__dirname, 'bindings'),
    },
  },
})
