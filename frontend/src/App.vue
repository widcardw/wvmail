<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView } from 'vue-router'
import { LocaleProvider } from '@ark-ui/vue/locale'
import { Splitter } from '@ark-ui/vue/splitter'
import { Events } from '@wailsio/runtime'
import { useRouter } from 'vue-router'
import { useThemeStore } from './stores/theme'
import { useLocaleStore } from './stores/locale'
import { usePreferencesStore } from './stores/preferences'
import { ConfigService } from '#/wvmail/services'
import TitleBar from './components/header/TitleBar.vue'
import SideBar from './components/sidebar/SideBar.vue'
import ToastHost from './components/ToastHost.vue'
import splitStyles from './components/mail-split.module.css'

const appRegistry = Splitter.createRegistry()

const theme = useThemeStore()
const localeStore = useLocaleStore()
const preferences = usePreferencesStore()
const router = useRouter()

onMounted(async () => {
  try {
    const config = await ConfigService.GetConfig()
    localeStore.loadFromConfig(config)
    theme.loadFromConfig(config) // also calls theme.apply()
    preferences.loadFromConfig(config)
  } catch (e) {
    console.error('Failed to load config', e)
  }
  theme.apply()

  // Listen for global keyboard shortcut to navigate to settings
  Events.On('navigate:settings', () => {
    router.push('/settings')
  })
})
</script>

<template>
  <LocaleProvider :locale="localeStore.current">
    <div class="app">
      <header class="app__titlebar">
        <TitleBar />
      </header>
      <div class="app__body">
        <Splitter.Root
          :class="splitStyles.root"
          :panels="[{ id: 'side', minSize: 8, maxSize: 34 }, { id: 'main' }]"
          :default-size="[16, 84]"
          :registry="appRegistry"
        >
          <Splitter.Panel :class="splitStyles.panel" id="side">
            <aside class="app-sider"><SideBar /></aside>
          </Splitter.Panel>

          <Splitter.ResizeTrigger
            :class="splitStyles.trigger"
            id="side:main"
            aria-label="resize sidebar"
          >
            <Splitter.ResizeTriggerIndicator :class="splitStyles.indicator" />
          </Splitter.ResizeTrigger>

          <Splitter.Panel :class="splitStyles.panel" id="main">
            <main class="app-content">
              <RouterView />
            </main>
          </Splitter.Panel>
        </Splitter.Root>
      </div>
      <ToastHost />
    </div>
  </LocaleProvider>
</template>

<style scoped>
.app {
  display: flex;
  flex-direction: column;
  height: 100%;
  color: var(--n-text-color);
  background: var(--n-body-color);
}

.app__titlebar {
  flex-shrink: 0;
  height: 40px;
  border-bottom: 1px solid var(--n-border-color);
}

.app__body {
  flex: 1;
  min-height: 0;
}
.app-sider,
.app-content {
  height: 100%;
  min-width: 0;
  overflow: hidden;
  background: var(--n-body-color);
}
.app-sider {
  background: var(--n-body-color);
}
</style>
