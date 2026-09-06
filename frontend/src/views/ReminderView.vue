<script setup lang="ts">
import { useI18n } from '~/i18n'

const { t } = useI18n()
</script>

<template>
  <div class="content">
    <section class="surface">
      <header class="surface__head">
        <h2>{{ t['reminders.title'] }}</h2>
      </header>
      <div class="empty">
        <i class="i-ri:notification-off-line empty__icon" aria-hidden="true" />
        <p>{{ t['reminders.empty'] }}</p>
      </div>
    </section>
  </div>
</template>

<style scoped>
.content {
  height: 100%;
  padding: 16px;
  overflow-y: auto;
  color: var(--n-text-color);
}
.surface {
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
  padding: 4px 0;
  max-width: 720px;
}
.surface__head {
  padding: 12px 16px;
  border-bottom: 1px solid var(--n-border-color);
}
.surface__head h2 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 48px 16px;
  color: var(--color-text-3);
  font-size: 13px;
}
.empty__icon {
  font-size: 34px;
  opacity: 0.7;
}
</style>
