<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Menu } from '@ark-ui/vue/menu'
import { useRouter } from 'vue-router'
import { ComposeService, OsService } from '#/wvmail/services'
import { useI18n } from '~/i18n'
import { useThemeStore, type ThemeMode } from '~/stores/theme'
import { useAccountsStore } from '~/stores/accounts'
import { useFeedbackStore } from '~/stores/feedback'
import SearchBar from './SearchBar.vue'
import ConnectionStatus from '~/components/icon/ConnectionStatus.vue'

const { t } = useI18n()
const router = useRouter()
const themeStore = useThemeStore()
const accountsStore = useAccountsStore()
const feedback = useFeedbackStore()

const historyTick = ref(0)
let stopAfterEach: (() => void) | undefined

const canGoBack = computed(() => {
  void historyTick.value
  return Boolean(router.options.history.state.back)
})

const canGoForward = computed(() => {
  void historyTick.value
  return Boolean(router.options.history.state.forward)
})

function goBack() {
  router.back()
}

function goForward() {
  router.forward()
}

const themeMode = computed<ThemeMode>({
  get: () => themeStore.mode,
  set: (mode) => {
    void themeStore.switchTo(mode)
  },
})

const themeOptions = computed<Array<{ value: ThemeMode; label: string; icon: string }>>(() => [
  { value: 'System', label: t.value['settings.common.theme.system'], icon: 'i-ri:computer-line' },
  { value: 'Light', label: t.value['settings.common.theme.light'], icon: 'i-ri:sun-line' },
  { value: 'Dark', label: t.value['settings.common.theme.dark'], icon: 'i-ri:moon-clear-line' },
])

const activeTheme = computed(
  () =>
    themeOptions.value.find((option) => option.value === themeStore.mode) ?? themeOptions.value[0],
)

// macOS traffic-light space
const lm = ref(0)
onMounted(() => {
  OsService.GetOs().then((os: string) => {
    if (os === 'darwin') lm.value = 80
  })
  stopAfterEach = router.afterEach(() => {
    historyTick.value++
  })
})

onUnmounted(() => {
  stopAfterEach?.()
})

async function openCompose() {
  const accountID = accountsStore.selectedAccountId ?? accountsStore.accounts[0]?.ID ?? ''
  try {
    const result = await ComposeService.OpenCompose({
      AccountID: accountID,
      To: [],
      Cc: [],
      Bcc: [],
      Subject: '',
      Text: '',
      HTML: '',
      InReplyTo: '',
      References: [],
    })
    if (result.Warning) feedback.info(result.Warning)
  } catch (e) {
    feedback.error(String(e))
  }
}
</script>

<template>
  <div class="titlebar">
    <div class="titlebar-left">
      <span class="brand" :style="{ marginLeft: `${lm}px` }">{{ t['app.brand'] }}</span>
      <div class="titlebar-nav">
        <button
          type="button"
          class="icon-btn"
          :disabled="!canGoBack"
          :title="t['app.back']"
          :aria-label="t['app.back']"
          @click="goBack"
        >
          <i class="i-ri:arrow-left-line" aria-hidden="true" />
        </button>
        <button
          type="button"
          class="icon-btn"
          :disabled="!canGoForward"
          :title="t['app.forward']"
          :aria-label="t['app.forward']"
          @click="goForward"
        >
          <i class="i-ri:arrow-right-line" aria-hidden="true" />
        </button>
      </div>
    </div>

    <div class="titlebar-center">
      <SearchBar />
    </div>

    <div class="titlebar-right">
      <button
        type="button"
        class="icon-btn"
        :title="t['compose.new']"
        :aria-label="t['compose.new']"
        @click="openCompose"
      >
        <i class="i-ri:quill-pen-line" aria-hidden="true" />
      </button>
      <ConnectionStatus />
      <Menu.Root :positioning="{ placement: 'bottom-end', gutter: 8 }" lazy-mount unmount-on-exit>
        <Menu.Trigger
          class="icon-btn theme-menu-trigger"
          :aria-label="activeTheme.label"
          :title="activeTheme.label"
        >
          <i :class="activeTheme.icon" aria-hidden="true" />
        </Menu.Trigger>
        <Teleport to="body">
          <Menu.Positioner class="theme-menu-positioner">
            <Menu.Content class="theme-menu-content">
              <Menu.RadioItemGroup v-model="themeMode">
                <Menu.RadioItem
                  v-for="option in themeOptions"
                  :key="option.value"
                  class="theme-menu-item"
                  :value="option.value"
                >
                  <i :class="option.icon" aria-hidden="true" />
                  <Menu.ItemText>{{ option.label }}</Menu.ItemText>
                  <Menu.ItemIndicator class="theme-menu-check">
                    <i class="i-ri:check-line" aria-hidden="true" />
                  </Menu.ItemIndicator>
                </Menu.RadioItem>
              </Menu.RadioItemGroup>
            </Menu.Content>
          </Menu.Positioner>
        </Teleport>
      </Menu.Root>
    </div>
  </div>
</template>

<style scoped>
.titlebar {
  display: grid;
  grid-template-columns: minmax(min-content, 1fr) minmax(0, 620px) minmax(min-content, 1fr);
  align-items: center;
  height: 40px;
  padding: 0 12px;
  gap: 12px;
}

.titlebar-left {
  display: flex;
  align-items: center;
  gap: 4px;
}

.brand {
  font-weight: 700;
  font-size: 14px;
  white-space: nowrap;
}

.titlebar-nav {
  display: flex;
  align-items: center;
  gap: 2px;
  margin-left: 4px;
}

.titlebar-nav .icon-btn:disabled:hover {
  background: transparent;
}

.titlebar-center {
  display: flex;
  justify-content: center;
  min-width: 0;
}

.titlebar-right {
  display: flex;
  align-items: center;
  gap: 8px;
  justify-self: end;
}

/* Base geometry comes from the shared .icon-btn (styles/ui.css); only the deltas
   live here: the open menu keeps the hover background, plus the padding reset the
   fixed 26px box wants. */
.theme-menu-trigger {
  padding: 0;
}
.theme-menu-trigger[data-state='open'] {
  background: var(--n-sidebar-hover);
}
.theme-menu-trigger:focus-visible {
  outline: 2px solid var(--n-primary-color);
  outline-offset: 1px;
}

.theme-menu-positioner {
  z-index: 1000;
}
.theme-menu-content {
  min-width: 132px;
  padding: 4px;
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
  box-shadow: 0 8px 20px rgb(0 0 0 / 18%);
}
.theme-menu-item {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr) 16px;
  align-items: center;
  gap: 8px;
  width: 100%;
  height: 30px;
  padding: 0 8px;
  color: var(--n-text-color);
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  outline: none;
  user-select: none;
}
.theme-menu-item[data-highlighted] {
  background: var(--n-sidebar-hover);
}
.theme-menu-item[data-state='checked'] {
  color: var(--n-primary-color);
}
.theme-menu-item[data-state='checked'][data-highlighted] {
  background: var(--n-sidebar-active);
}
.theme-menu-item > i,
.theme-menu-check i {
  font-size: 16px;
  line-height: 1;
}
.theme-menu-check {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.theme-menu-check[data-state='unchecked'] {
  visibility: hidden;
}
</style>
