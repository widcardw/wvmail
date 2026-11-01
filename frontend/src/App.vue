<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { RouterView } from 'vue-router'
import { LocaleProvider } from '@ark-ui/vue/locale'
import { Splitter } from '@ark-ui/vue/splitter'
import { Events } from '@wailsio/runtime'
import { useRouter } from 'vue-router'
import { useRoute } from 'vue-router'
import { useThemeStore } from './stores/theme'
import { useLocaleStore } from './stores/locale'
import { usePreferencesStore } from './stores/preferences'
import { useFeedbackStore } from './stores/feedback'
import { useAccountsStore } from './stores/accounts'
import { useI18n } from './i18n'
import { ComposeService, ConfigService } from '#/wvmail/services'
import TitleBar from './components/header/TitleBar.vue'
import SideBar from './components/sidebar/SideBar.vue'
import ToastHost from './components/ToastHost.vue'
import splitStyles from './components/mail-split.module.css'

const theme = useThemeStore()
const localeStore = useLocaleStore()
const preferences = usePreferencesStore()
const feedback = useFeedbackStore()
const accountsStore = useAccountsStore()
const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const isCompose = computed(() => route.name === 'compose')
let unlistenComposeSent: (() => void) | null = null

async function handleApplicationMailto(event: MouseEvent) {
  if (isCompose.value || event.defaultPrevented || event.button !== 0) return
  const path = event.composedPath()
  const anchor = path.find((node): node is HTMLAnchorElement => node instanceof HTMLAnchorElement)
  const href = anchor?.getAttribute('href') ?? ''
  if (!href.toLowerCase().startsWith('mailto:')) return
  event.preventDefault()
  try {
    const result = await ComposeService.OpenMailto(
      href,
      accountsStore.selectedEmailAccountId ?? accountsStore.selectedAccountId ?? '',
    )
    if (result.Warning) feedback.info(result.Warning)
  } catch (e) {
    feedback.error(t.value['compose.open_failed'])
    console.error('Failed to open application mailto link', e)
  }
}

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
  document.addEventListener('click', handleApplicationMailto)

  // Listen for global keyboard shortcut to navigate to settings
  Events.On('navigate:settings', () => {
    if (isCompose.value) return
    router.push('/settings')
  })

  unlistenComposeSent = Events.On('compose:sent', (event) => {
    const payload = event.data as { warning?: string } | undefined
    if (payload?.warning) feedback.error(payload.warning)
    else feedback.success(t.value['compose.sent'])
  })
})

onUnmounted(() => {
  unlistenComposeSent?.()
  document.removeEventListener('click', handleApplicationMailto)
})
</script>

<template>
  <LocaleProvider :locale="localeStore.current">
    <RouterView v-if="isCompose" />
    <div v-else class="app">
      <header class="app__titlebar">
        <TitleBar />
      </header>
      <div class="app__body">
        <Splitter.Root
          :class="splitStyles.root"
          :panels="[{ id: 'side', minSize: 11, maxSize: 34 }, { id: 'main' }]"
          :default-size="[16, 84]"
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
