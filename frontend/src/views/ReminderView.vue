<script setup lang="ts">
import { NCard, NEmpty } from 'naive-ui'
import { useI18n } from '../i18n'

const { t } = useI18n()
</script>

<template>
  <div class="view-content">
    <NCard :title="t['reminders.title']">
      <NEmpty :description="t['reminders.empty']" />
    </NCard>
  </div>
</template>

<style scoped>
.view-content {
  flex: 1;
  padding: 16px;
  overflow-y: auto;
}
</style>
