import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import wails from '@wailsio/runtime/plugins/vite'
import { fileURLToPath, URL } from 'node:url'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue(), wails('./bindings')],
  server: {
    // wails3 backend failed to dial 127.0.0.1:9245,
    // since by default the server starts at `localhost` instead of `127.0.0.1`
    host: '127.0.0.1',
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
})
