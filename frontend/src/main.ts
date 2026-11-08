import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router from './router'
import App from './App.vue'
import 'uno.css'
import './styles/global.css'
import './styles/ui.css'
import { useThemeStore } from './stores/theme'
import { i18n } from './i18n'

const pinia = createPinia()

const app = createApp(App)
app.use(pinia)
app.use(i18n)
app.use(router)

// Apply theme/data-theme before first paint to avoid flash.
useThemeStore(pinia).apply()

app.mount('#app')
