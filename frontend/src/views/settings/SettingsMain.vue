<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from '~/i18n'
import UiScrollArea from '~/components/ui/UiScrollArea.vue'

const router = useRouter()
const { t } = useI18n()

const items = computed(() => [
  { key: 'common', label: t.value['settings.common.label'], icon: 'ri:home-2-line' },
  { key: 'accounts', label: t.value['settings.accounts.label'], icon: 'ri:account-circle-2-line' },
  { key: 'contacts', label: t.value['contacts.title'], icon: 'ri:contacts-book-2-line' },
  { key: 'security', label: t.value['settings.security.label'], icon: 'ri:shield-check-line' },
  { key: 'proxy', label: t.value['settings.proxy.label'], icon: 'ri:route-line' },
  { key: 'llm', label: t.value['settings.llm.label'], icon: 'ri:robot-2-line' },
  { key: 'tags', label: t.value['settings.tags'], icon: 'ri:price-tag-3-line' },
  { key: 'ai-assistant', label: t.value['settings.aiAssistant.label'], icon: 'ri:sparkling-line' },
  { key: 'about', label: t.value['settings.about.label'], icon: 'ri:information-line' },
])

function isActive(key: string) {
  return router.currentRoute.value.path.endsWith(`/${key}`)
}
function go(key: string) {
  if (!isActive(key)) router.push(`/settings/${key}`)
}
</script>

<template>
  <div class="settings">
    <nav class="settings__nav">
      <button
        v-for="item in items"
        :key="item.key"
        type="button"
        class="nav-item"
        :class="{ active: isActive(item.key) }"
        @click="go(item.key)"
      >
        <i :class="`i-${item.icon}`" aria-hidden="true" />
        <span>{{ item.label }}</span>
      </button>
    </nav>
    <div class="settings__body">
      <UiScrollArea class="settings__scroll">
        <RouterView />
      </UiScrollArea>
    </div>
  </div>
</template>

<style scoped>
.settings {
  height: calc(100vh - 40px);
  display: flex;
  color: var(--n-text-color);
}
.settings__nav {
  width: 160px;
  flex-shrink: 0;
  padding: 8px 6px;
  border-right: 1px solid var(--n-border-color);
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  font-size: 13px;
  border: none;
  background: transparent;
  border-radius: 6px;
  cursor: pointer;
  text-align: left;
  color: var(--n-text-color);
}
.nav-item:hover {
  background: var(--n-sidebar-hover);
}
.nav-item.active {
  background: var(--n-sidebar-active);
}
.nav-item i {
  flex: 0 0 16px;
  font-size: 16px;
}
.settings__body {
  flex: 1;
  min-width: 0;
  overflow: hidden;
}
.settings__scroll {
  height: 100%;
}
</style>
