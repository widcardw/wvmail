<script setup lang="ts">
import { Tooltip } from '@ark-ui/vue/tooltip'

/**
 * One icon-only action with an Ark tooltip.
 *
 * The native `title` attribute used to be the only label on these buttons. It
 * waits a second, cannot be styled, and on a row of five glyphs gives no hint
 * which one is "translate". This primitive keeps the icon geometry in one place
 * while every caller supplies the accessible name and the action.
 */
const props = withDefaults(
  defineProps<{
    label: string
    icon: string
    disabled?: boolean
    /** A spinner replaces the glyph while the action is running. */
    busy?: boolean
    /** Keep the button clickable while busy, for actions that cancel themselves. */
    busyClickable?: boolean
    active?: boolean
    /** Match the neutral sidebar selection instead of the warning-tinted action. */
    activeTone?: 'warning' | 'sidebar'
    danger?: boolean
    /** Preferred tooltip side; the sidebar uses bottom for its top row and top below. */
    placement?: 'top' | 'bottom' | 'left' | 'right'
  }>(),
  {
    disabled: false,
    busy: false,
    busyClickable: false,
    active: false,
    activeTone: 'warning',
    danger: false,
    placement: 'bottom',
  },
)

const emit = defineEmits<{
  click: []
}>()
</script>

<template>
  <Tooltip.Root :open-delay="250" :close-delay="80" :positioning="{ placement: props.placement }">
    <Tooltip.Trigger as-child>
      <button
        type="button"
        class="icon-btn detail-action"
        :class="{
          'detail-action--active': active && activeTone === 'warning',
          'detail-action--active-sidebar': active && activeTone === 'sidebar',
          'detail-action--danger': danger,
        }"
        :disabled="disabled || (busy && !busyClickable)"
        :aria-label="label"
        :aria-pressed="active || undefined"
        @click="emit('click')"
      >
        <span v-if="busy" class="detail-action__spin" aria-hidden="true" />
        <i v-else :class="`i-${icon}`" aria-hidden="true" />
      </button>
    </Tooltip.Trigger>
    <Teleport to="body">
      <Tooltip.Positioner class="detail-tip-positioner">
        <Tooltip.Content class="detail-tip">{{ label }}</Tooltip.Content>
      </Tooltip.Positioner>
    </Teleport>
  </Tooltip.Root>
</template>

<style scoped>
.detail-action {
  width: 28px;
  height: 28px;
}
.detail-action--active {
  color: var(--color-warning);
  background: var(--color-warning-bg);
}
.detail-action--active-sidebar {
  color: var(--n-text-color);
  background: var(--n-sidebar-active);
}
.detail-action--danger {
  color: var(--color-danger);
}
.detail-action--danger:hover {
  background: var(--color-danger-bg);
}
.detail-action__spin {
  width: 13px;
  height: 13px;
  border: 2px solid var(--n-border-color);
  border-top-color: var(--n-primary-color);
  border-radius: 50%;
  animation: detail-action-spin 0.8s linear infinite;
}
@keyframes detail-action-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>

<style>
.detail-tip-positioner {
  z-index: 1000;
}
.detail-tip {
  padding: 5px 8px;
  font-size: 12px;
  line-height: 1.35;
  color: var(--n-text-color);
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
  box-shadow: 0 8px 20px rgb(0 0 0 / 18%);
}
</style>
