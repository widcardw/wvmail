<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Menu } from '@ark-ui/vue/menu'
import { OsService } from '#/wvmail/services'
import { useI18n } from '~/i18n'
import { useThemeStore, type ThemeMode } from '~/stores/theme'
import SearchBar from './SearchBar.vue'

const { t } = useI18n()
const themeStore = useThemeStore()

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
})
</script>

<template>
  <div class="titlebar">
    <div class="titlebar-left">
      <span class="brand" :style="{ marginLeft: `${lm}px` }">{{ t['app.brand'] }}</span>
    </div>

    <div class="titlebar-center">
      <SearchBar />
    </div>

    <div class="titlebar-right">
      <Menu.Root :positioning="{ placement: 'bottom-end', gutter: 8 }" lazy-mount unmount-on-exit>
        <Menu.Trigger
          class="theme-menu-trigger"
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
  display: flex;
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

.titlebar-center {
  flex: 1;
  display: flex;
  justify-content: center;
}

.titlebar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.theme-menu-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  padding: 0;
  color: var(--n-text-color);
  background: transparent;
  border: none;
  border-radius: 5px;
  cursor: pointer;
}
.theme-menu-trigger:hover,
.theme-menu-trigger[data-state='open'] {
  background: var(--n-sidebar-hover);
}
.theme-menu-trigger:focus-visible {
  outline: 2px solid var(--n-primary-color);
  outline-offset: 1px;
}
.theme-menu-trigger i {
  font-size: 16px;
  line-height: 1;
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
