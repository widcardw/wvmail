<script setup lang="ts">
import { useFeedbackStore } from '~/stores/feedback'

const feedback = useFeedbackStore()
</script>

<template>
  <div class="toast-host" role="status" aria-live="polite">
    <TransitionGroup name="toast">
      <div v-for="t in feedback.toasts" :key="t.id" class="toast" :class="`toast--${t.kind}`">
        {{ t.text }}
        <button
          type="button"
          class="toast__close"
          aria-label="close"
          @click="feedback.dismiss(t.id)"
        >
          <i class="i-ri:close-line" />
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-host {
  position: fixed;
  top: 48px;
  right: 16px;
  z-index: 1000;
  display: flex;
  flex-direction: column;
  gap: 8px;
  pointer-events: none;
}
.toast {
  pointer-events: auto;
  display: flex;
  align-items: center;
  gap: 8px;
  max-width: 320px;
  padding: 9px 12px;
  font-size: 13px;
  color: var(--n-text-color);
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-left-width: 3px;
  border-radius: 6px;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.15);
}
.toast--success {
  border-left-color: var(--success, #18a058);
}
.toast--error {
  border-left-color: var(--danger, #d03050);
}
.toast--warn {
  border-left-color: var(--warning, #f0a020);
}
.toast--info {
  border-left-color: var(--info, #2080f0);
}
.toast__close {
  border: none;
  background: none;
  cursor: pointer;
  margin-left: auto;
  color: var(--n-text-color);
  opacity: 0.7;
}
.toast__close i {
  font-size: 14px;
}
.toast-enter-active,
.toast-leave-active {
  transition:
    opacity 0.15s,
    transform 0.15s;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>
