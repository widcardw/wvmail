<script setup lang="ts">
import { NCard } from 'naive-ui'
import { useI18n } from '../i18n'

const { t } = useI18n()
</script>

<template>
  <div class="view-content">
    <div class="content-placeholder">
      <NCard>
        <p style="opacity: 0.5; text-align: center; padding: 4rem 0">
          {{ t['mail.selectFolder'] }}
        </p>
      </NCard>
    </div>
  </div>
</template>

<style scoped>
.view-content {
  flex: 1;
  padding: 16px;
  overflow-y: auto;
}

.content-placeholder {
  max-width: 640px;
  margin: 0 auto;
}
</style>
