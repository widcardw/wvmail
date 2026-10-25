<script setup lang="ts">
import { Dialog } from '@ark-ui/vue/dialog'

/**
 * The shared Ark dialog shell: backdrop, positioner, and card with a standard
 * head/body/foot layout. This replaces the seven hand-copied
 * `Dialog.Root → Backdrop → Positioner → Content` blocks across the app.
 *
 * The caller owns `open`; the content inside each slot is entirely the caller's.
 */
const props = withDefaults(
  defineProps<{
    open: boolean
    /** `narrow` (400px), `default` (440px), `wide` (720px), `top` (scroll), or `crop` (520px). */
    size?: 'narrow' | 'default' | 'wide' | 'top' | 'crop'
    /** ARIA role, passed through to Ark. */
    role?: 'dialog' | 'alertdialog'
    /** Set while the caller's action runs: stops Escape/outside-click dismissal. */
    busy?: boolean
  }>(),
  { size: 'default', role: undefined, busy: false },
)

const emit = defineEmits<{ 'update:open': [open: boolean] }>()

function onOpenChange(open: boolean) {
  if (!open && props.busy) return
  emit('update:open', open)
}

const cardClass = {
  narrow: 'dlg-card dlg-card--narrow',
  default: 'dlg-card',
  wide: 'dlg-card dlg-card--wide dlg-card--flush',
  top: 'dlg-card dlg-card--wide dlg-card--flush',
  crop: 'dlg-card dlg-card--crop dlg-card--flush',
} as const
</script>

<template>
  <Dialog.Root
    :open="props.open"
    :role="props.role"
    :close-on-escape="!props.busy"
    :close-on-interact-outside="!props.busy"
    lazy-mount
    unmount-on-exit
    @update:open="onOpenChange"
  >
    <Teleport to="body">
      <Dialog.Backdrop class="dlg-back" />
      <Dialog.Positioner class="dlg-pos" :class="{ 'dlg-pos--top': props.size === 'top' }">
        <Dialog.Content :class="cardClass[props.size]">
          <slot />
        </Dialog.Content>
      </Dialog.Positioner>
    </Teleport>
  </Dialog.Root>
</template>
