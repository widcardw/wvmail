<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { OsService } from '#/wvmail/services'
import { useI18n } from '~/i18n'
import { useThemeStore } from '~/stores/theme'

const { t } = useI18n()
const themeStore = useThemeStore()

const isDark = computed(() => themeStore.effectiveTheme === 'dark')

function toggleTheme() {
  // Persist to config: flip effective light/dark (System stays selectable in Settings).
  void themeStore.switchTo(isDark.value ? 'Light' : 'Dark')
}

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
      <input class="search-input" type="search" :placeholder="t['search.placeholder']" />
    </div>

    <div class="titlebar-right">
      <button
        type="button"
        class="icon-btn"
        role="switch"
        :aria-checked="isDark"
        :title="isDark ? 'Switch to light' : 'Switch to dark'"
        @click="toggleTheme"
      >
        <template v-if="isDark">
          <i class="i-ri:moon-clear-line" aria-hidden="true" />
        </template>
        <template v-else>
          <i class="i-ri:sun-line" aria-hidden="true" />
        </template>
      </button>
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
  height: 26px;
  padding: 0 10px;
  font-size: 13px;
  color: var(--n-text-color);
  background: var(--action-color, var(--n-action-color));
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  outline: none;
}
.search-input:focus {
  border-color: var(--n-primary-color);
}

.titlebar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  color: var(--n-text-color);
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}
.icon-btn:hover {
  background: var(--n-border-color);
}

.icon-btn i {
  font-size: 16px;
  line-height: 1;
}
</style>
