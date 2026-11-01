<script setup lang="ts">
import { computed } from 'vue'

type AlertTone = 'info' | 'success' | 'warning' | 'error'

const props = withDefaults(
  defineProps<{
    tone?: AlertTone
    dismissible?: boolean
    role?: 'alert' | 'status'
    closeLabel?: string
    /** Override the tone's default Remix icon class. */
    icon?: string
  }>(),
  {
    tone: 'info',
    dismissible: false,
    role: undefined,
    closeLabel: 'Close',
    icon: '',
  },
)

const emit = defineEmits<{ dismiss: [] }>()

const toneIcon = computed(() => {
  switch (props.tone) {
    case 'success':
      return 'i-ri:checkbox-circle-line'
    case 'warning':
      return 'i-ri:error-warning-line'
    case 'error':
      return 'i-ri:close-circle-line'
    default:
      return 'i-ri:information-line'
  }
})

const resolvedRole = computed(() => props.role ?? (props.tone === 'error' ? 'alert' : 'status'))
</script>

<template>
  <div class="ui-alert" :class="`ui-alert--${props.tone}`" :role="resolvedRole">
    <i class="ui-alert__icon" :class="props.icon || toneIcon" aria-hidden="true" />
    <div class="ui-alert__content"><slot /></div>
    <button
      v-if="props.dismissible"
      type="button"
      class="ui-alert__close"
      :aria-label="props.closeLabel"
      @click="emit('dismiss')"
    >
      <i class="i-ri:close-line" aria-hidden="true" />
    </button>
  </div>
</template>

<style scoped>
.ui-alert {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 9px 12px;
  color: var(--n-text-color);
  background: var(--color-surface-2);
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  font-size: 13px;
  line-height: 1.5;
}
.ui-alert__icon {
  flex: none;
  margin-top: 1px;
  color: var(--color-text-2);
  font-size: 15px;
}
.ui-alert__content {
  flex: 1;
  min-width: 0;
}
.ui-alert__close {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  padding: 0;
  color: var(--color-text-2);
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}
.ui-alert__close:hover {
  color: var(--n-text-color);
  background: var(--n-sidebar-hover);
}
.ui-alert__close i {
  font-size: 14px;
}
.ui-alert--success {
  background: var(--color-success-bg);
  border-color: color-mix(in srgb, var(--color-success) 38%, transparent);
}
.ui-alert--success .ui-alert__icon {
  color: var(--color-success);
}
.ui-alert--warning {
  background: var(--color-warning-bg);
  border-color: color-mix(in srgb, var(--color-warning) 38%, transparent);
}
.ui-alert--warning .ui-alert__icon {
  color: var(--color-warning);
}
.ui-alert--error {
  background: var(--color-danger-bg);
  border-color: color-mix(in srgb, var(--color-danger) 42%, transparent);
}
.ui-alert--error .ui-alert__icon {
  color: var(--color-danger);
}
.ui-alert--info .ui-alert__icon {
  color: var(--n-primary-color);
}
</style>
