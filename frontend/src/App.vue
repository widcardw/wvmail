<script setup lang="ts">
import {
  NConfigProvider,
  NDialogProvider,
  NLayout,
  NLayoutContent,
  NLayoutHeader,
  NLayoutSider,
  NMessageProvider,
} from 'naive-ui'
import { RouterView, useRouter } from 'vue-router'
import { useThemeStore } from './stores/theme'
import { useLocaleStore } from './stores/locale'
import ThemeStyles from './components/ThemeStyles.vue'
import TitleBar from './components/TitleBar.vue'
import SideBar from './components/SideBar.vue'
import { onMounted, ref } from 'vue'
import { ConfigService } from '#/wvmail/services'
import { Events } from '@wailsio/runtime'

const theme = useThemeStore()
const localeStore = useLocaleStore()
const collapsed = ref(false)
const router = useRouter()

onMounted(async () => {
  try {
    const config = await ConfigService.GetConfig()
    localeStore.loadFromConfig(config)
    theme.loadFromConfig(config)
  } catch (e) {
    console.error('Failed to load config', e)
  }

  // Listen for global keyboard shortcut to navigate to settings
  Events.On('navigate:settings', () => {
    router.push('/settings')
  })
})
</script>

<template>
  <NConfigProvider
    :theme="theme.naiveTheme"
    :theme-overrides="theme.themeOverrides"
    :locale="localeStore.naiveLocale"
    :date-locale="localeStore.naiveDateLocale"
    style="height: 100%"
  >
    <NMessageProvider>
      <NDialogProvider>
        <ThemeStyles />
        <NLayout style="height: 100%">
          <NLayoutHeader bordered>
            <TitleBar v-model:collapsed="collapsed" />
          </NLayoutHeader>
          <NLayout has-sider class="n-full">
            <NLayoutSider bordered :collapsed="collapsed" collapse-mode="width" width="180">
              <SideBar :collapsed="collapsed" />
            </NLayoutSider>
            <NLayoutContent>
              <RouterView />
            </NLayoutContent>
          </NLayout>
        </NLayout>
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>

<style>
html,
body,
#app {
  margin: 0;
  padding: 0;
  height: 100%;
  overflow: hidden;
}
</style>

<style scoped>
.n-full {
  height: calc(100vh - 41px);
}
</style>
