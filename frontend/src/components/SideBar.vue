<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { NScrollbar, NMenu } from 'naive-ui'
import type { MenuOption } from 'naive-ui'
import { ICON_MAPPING } from '../composables/icon-mapping'
import { useI18n } from '../i18n'

const { t } = useI18n()

const menuOptions = computed<MenuOption[]>(() => [
  {
    label: 'lee@gmail.com',
    key: 'lee@gmail.com',
    icon: () => ICON_MAPPING['gmail'].create(),
    children: [
      { label: t.value['sidebar.inbox'], key: 'lee-inbox', icon: () => '📥' },
      { label: t.value['sidebar.sent'], key: 'lee-sent', icon: () => '📤' },
    ],
  },
  {
    label: 'work@11.com',
    key: 'work@11.com',
    icon: () => ICON_MAPPING['qq'].create(),
    children: [
      { label: t.value['sidebar.inbox'], key: 'work-inbox', icon: () => '📥' },
      { label: t.value['sidebar.sent'], key: 'work-sent', icon: () => '📤' },
    ],
  },
  {
    label: 'work@163.com',
    key: 'work@163.com',
    icon: () => ICON_MAPPING['163'].create(),
    children: [
      { label: t.value['sidebar.inbox'], key: '163-inbox', icon: () => '📥' },
      { label: t.value['sidebar.sent'], key: '163-sent', icon: () => '📤' },
    ],
  },
  {
    label: 'work@outlook.com',
    key: 'work@outlook.com',
    icon: () => ICON_MAPPING['outlook'].create(),
    children: [
      { label: t.value['sidebar.inbox'], key: 'outlook-inbox', icon: () => '📥' },
      { label: t.value['sidebar.sent'], key: 'outlook-sent', icon: () => '📤' },
    ],
  },
])

const routeMap: Record<string, string> = {
  notes: '/notes',
  reminder: '/reminder',
  setting: '/settings',
}

function handleUpdateValue(key: string, _item: MenuOption) {
  // if (key === 'theme') {
  //   theme.switchMode()
  //   return
  // }
  // const path = routeMap[key]
  // if (path) {
  //   router.push(path)
  // }
}
</script>

<template>
  <NScrollbar>
    <NMenu
      :root-indent="16"
      :indent="16"
      :icon-size="24"
      :collapsed-icon-size="24"
      accordion
      :options="menuOptions"
      @update:value="handleUpdateValue"
    />
  </NScrollbar>
</template>

<style scoped></style>
