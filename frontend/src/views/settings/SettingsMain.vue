<script setup lang="ts">
import { NLayout, NLayoutContent, NLayoutSider, NMenu, type MenuOption } from 'naive-ui'
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from '~/i18n'
import { useIconRenderer } from '~/composables/use-icon'
const router = useRouter()
const { t } = useI18n()
const { renderIcon } = useIconRenderer()
const settingsMenuOptions = computed(() => [
  {
    label: t.value['settings.common'],
    key: 'common',
    icon: renderIcon('ri:home-3-line'),
  },
  {
    label: t.value['settings.accounts'],
    key: 'accounts',
    icon: renderIcon('ri:account-circle-2-line'),
  },
  {
    label: t.value['settings.about'],
    key: 'about',
    icon: renderIcon('ri:information-line'),
  },
  {
    label: t.value['settings.llm'],
    key: 'llm',
    icon: renderIcon('ri:robot-2-line'),
  },
  {
    label: t.value['settings.translation'],
    key: 'translation',
    icon: renderIcon('ri:translate-2'),
  },
])

function handleClick(key: string, _item: MenuOption) {
  router.push(`/settings/${key}`)
}
</script>

<template>
  <NLayout has-sider class="n-full">
    <NLayoutSider :width="160" bordered>
      <NMenu
        :options="settingsMenuOptions"
        :root-indent="16"
        :indent="16"
        :icon-size="24"
        @update-value="handleClick"
      />
    </NLayoutSider>
    <NLayoutContent>
      <RouterView />
    </NLayoutContent>
  </NLayout>
</template>

<style scoped>
.n-full {
  height: calc(100vh - 41px);
}
</style>
