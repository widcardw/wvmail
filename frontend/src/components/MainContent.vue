<script setup lang="ts">
import { NCard } from 'naive-ui'
</script>

<template>
  <div class="main-content">
    <div class="content-placeholder">
      <NCard>
        <p style="opacity: 0.5; text-align: center; padding: 4rem 0">
          Select a folder to view emails
        </p>
      </NCard>
    </div>
  </div>
</template>

<style scoped>
.main-content {
  flex: 1;
  padding: 16px;
  overflow-y: auto;
}

.content-placeholder {
  max-width: 640px;
  margin: 0 auto;
}
</style>
