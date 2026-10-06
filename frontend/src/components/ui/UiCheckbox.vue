<script setup lang="ts">
import { Checkbox } from '@ark-ui/vue/checkbox'

// Thin wrapper around Ark's Checkbox so every tick box in the app shares one
// visual definition (the `.checkbox` / `.checkbox__indicator` parts live in
// styles/ui.css). Like UiSwitch, the Ark root is a <label>, so passing a label
// through the slot makes the whole row toggle.
defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    checked: boolean
    disabled?: boolean
    /** Class for the Ark label part — reuse the caller's own label class. */
    labelClass?: string
  }>(),
  { disabled: false, labelClass: '' },
)

const emit = defineEmits<{ 'update:checked': [checked: boolean] }>()
</script>

<template>
  <Checkbox.Root
    v-bind="$attrs"
    :checked="props.checked"
    :disabled="props.disabled"
    @checked-change="emit('update:checked', $event.checked === true)"
  >
    <Checkbox.Control class="checkbox">
      <Checkbox.Indicator class="checkbox__indicator">
        <i class="i-ri:check-line" aria-hidden="true" />
      </Checkbox.Indicator>
    </Checkbox.Control>
    <Checkbox.Label v-if="$slots.default" :class="props.labelClass">
      <slot />
    </Checkbox.Label>
    <Checkbox.HiddenInput />
  </Checkbox.Root>
</template>
