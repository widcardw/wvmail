<script setup lang="ts">
import { NInput, NText, NButton, NSwitch } from 'naive-ui'
import { OsService } from '#/wvmail/services'
import { computed, onMounted, ref } from 'vue'
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
      <NInput size="small" :placeholder="t['search.placeholder']" clearable class="search-input" />
    </div>

    <div class="titlebar-right">
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
</style>
