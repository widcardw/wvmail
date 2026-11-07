<script setup lang="ts">
import { Tooltip } from '@ark-ui/vue/tooltip'

/**
 * The single tooltip primitive for icon-only controls.
 *
 * Native `title` used to be the label on the note/compose toolbars while the mail
 * action bar already used Ark's Tooltip: different delays, different styling, and
 * a `title` can be neither themed nor kept out of the way of a scrolling toolbar.
 * Every caller now wraps its trigger in this component (the content is teleported
 * to <body>, so the toolbar's `overflow-x: auto` cannot clip it).
 *
 * The trigger must be a single element, which is what `as-child` gives us.
 */
withDefaults(
  defineProps<{
    label: string
    placement?: 'top' | 'bottom' | 'left' | 'right'
    openDelay?: number
    closeDelay?: number
    disabled?: boolean
  }>(),
  {
    placement: 'bottom',
    openDelay: 250,
    closeDelay: 80,
    disabled: false,
  },
)
</script>

<template>
  <Tooltip.Root
    :open-delay="openDelay"
    :close-delay="closeDelay"
    :disabled="disabled"
    :positioning="{ placement }"
  >
    <Tooltip.Trigger as-child>
      <slot />
    </Tooltip.Trigger>
    <Teleport to="body">
      <Tooltip.Positioner class="ui-tip-positioner">
        <Tooltip.Content class="ui-tip">{{ label }}</Tooltip.Content>
      </Tooltip.Positioner>
    </Teleport>
  </Tooltip.Root>
</template>

<style>
.ui-tip-positioner {
  z-index: 1200;
}
.ui-tip {
  max-width: 260px;
  padding: 5px 8px;
  color: var(--n-text-color);
  font-size: 12px;
  line-height: 1.35;
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
  box-shadow: 0 8px 20px rgb(0 0 0 / 18%);
}
</style>
