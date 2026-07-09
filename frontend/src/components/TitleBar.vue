<script setup lang="ts">
import { NInput, NText, NButton, NSwitch, NMenu, MenuOption } from 'naive-ui'
import { OsService } from '#/wvmail/services'
import { computed, h, onMounted, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from '~/i18n'
import { useThemeStore } from '~/stores/theme'
import AppIcon from './AppIcon.vue'

const { t } = useI18n()
const themeStore = useThemeStore()

const isDark = computed(() => themeStore.naiveTheme !== null)

function toggleTheme() {
  themeStore.mode = isDark.value ? 'Light' : 'Dark'
}

const lm = ref(0)
onMounted(() => {
  OsService.GetOs().then((os: string) => {
    if (os === 'darwin') {
      lm.value = 80
    }
  })
})

const collapseMode = defineModel('collapsed', { type: Boolean, default: false })

const route = useRoute()

const activeKey = computed(() => {
  const path = route.path
  if (path.startsWith('/notes')) return 'notes'
  if (path.startsWith('/reminder')) return 'reminder'
  if (path.startsWith('/settings')) return 'settings'
  return null
})

const titleMenu: MenuOption[] = [
  {
    label: () => h(RouterLink, { to: '/settings' }),
    icon: () => h(AppIcon, { icon: 'ri:settings-2-line' }),
    key: 'settings',
  },
  {
    label: () => h(RouterLink, { to: '/reminder' }),
    icon: () => h(AppIcon, { icon: 'ri:notification-2-line' }),
    key: 'reminder',
  },
  {
    label: () => h(RouterLink, { to: '/notes' }),
    icon: () => h(AppIcon, { icon: 'ri:pen-nib-fill' }),
    key: 'notes',
  },
]
</script>

<template>
  <div class="titlebar">
    <div class="titlebar-left">
      <NText class="brand" :style="{ marginLeft: `${lm}px` }">{{ t['app.brand'] }}</NText>
      <NButton text size="small" @click="collapseMode = !collapseMode">
        <template #icon>
          <AppIcon icon="material-symbols:dock-to-left-outline" />
        </template>
      </NButton>
    </div>

    <div class="titlebar-center">
      <NInput
        size="small"
        :placeholder="t['search.placeholder']"
        clearable
        class="search-input"
      />
    </div>

    <div class="titlebar-right">
      <NMenu class="menu-horizon" :options="titleMenu" mode="horizontal" :icon-size="18" :value="activeKey" />
      <div class="separator" />
      <NSwitch :value="isDark" size="small" @update:value="toggleTheme">
        <template #checked-icon>
          <AppIcon icon="ri:moon-clear-line" :size="11" />
        </template>
        <template #unchecked-icon>
          <AppIcon icon="ri:sun-line" :size="11" />
        </template>
      </NSwitch>
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

.search-input {
  max-width: 380px;
  width: 100%;
}

.titlebar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.separator {
  width: 1px;
  height: 18px;
  background: var(--n-border-color);
  opacity: 0.4;
}

.menu-horizon:deep(.n-menu-item-content) {
  padding: 0 0.2rem;
}

.menu-horizon:deep(.n-menu-item-content .n-icon) {
  font-size: 18px;
}
</style>
