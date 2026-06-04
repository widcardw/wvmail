<script setup lang="ts">
import { NCard } from 'naive-ui'
import { useI18n } from '../../i18n'

const { t } = useI18n()
</script>

<template>
  <div style="padding: 16px; max-width: 720px">
    <NCard :title="t['settings.about']">
      <p style="opacity: 0.6">wvmail — Cross-platform desktop email client built with Wails3 + Vue 3</p>
      <p style="opacity: 0.4; font-size: 12px; margin-top: 8px">Version 0.0.1</p>
    </NCard>
  </div>
</template>
