<script setup lang="ts">
import { NInput, NText, NFlex, NButton, NIcon, NButtonGroup, MenuOption, NMenu } from 'naive-ui'
import { OsService } from '../../bindings/wvmail/services'
import { h, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { Icon } from '@iconify/vue'
import { useI18n } from '../i18n'

const { t } = useI18n()
const lm = ref(0)
onMounted(() => {
  OsService.GetOs().then((os: string) => {
    if (os === 'darwin') {
      lm.value = 80
    }
  })
})

const collapseMode = defineModel('collapsed', { type: Boolean, default: false })

const titleMenu: MenuOption[] = [
  {
    label: () => h(RouterLink, { to: '/settings' }),
    icon: () => h(NIcon, null, { default: () => h(Icon, { icon: 'ri:settings-2-line' }) }),
    key: 'settings',
  },
  {
    label: () => h(RouterLink, { to: '/reminder' }),
    icon: () => h(NIcon, null, { default: () => h(Icon, { icon: 'ri:notification-2-line' }) }),
    key: 'reminder',
  },
  {
    label: () => h(RouterLink, { to: '/notes' }),
    icon: () => h(NIcon, null, { default: () => h(Icon, { icon: 'ri:pen-nib-fill' }) }),
    key: 'notes',
  },
]
</script>

<template>
  <NFlex inline justify-center :wrap="false" style="height: 40px; align-items: center">
    <NText class="title-bar-brand" :style="{ marginLeft: `${lm}px` }">{{ t['app.brand'] }}</NText>
    <NButton text size="small" @click="collapseMode = !collapseMode">
      <template #icon>
        <NIcon><Icon icon="material-symbols:dock-to-left-outline" /></NIcon>
      </template>
    </NButton>
    <div>
      <NInput size="small" style="width: 400px" :placeholder="t['search.placeholder']" clearable />
    </div>
    <NMenu :options="titleMenu" mode="horizontal" :icon-size="20" />
  </NFlex>
</template>

<style scoped>
.icon-color {
  color: var(--n-text-color);
}
.title-bar-brand {
  font-weight: 700;
  font-size: 15px;
  white-space: nowrap;
}
</style>
