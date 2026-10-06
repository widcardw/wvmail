<script setup lang="ts">
import { Switch } from '@ark-ui/vue/switch'

// Thin wrapper around Ark's Switch so every toggle in the app shares one visual
// definition (the `.switch` / `.switch__thumb` parts live in styles/ui.css).
//
// Callers normally keep their own row markup and pass the label through the
// default slot: the Ark root is a <label>, so the whole row toggles. Pass a bare
// `<UiSwitch />` (no slot) when the label has to stay outside, e.g. in a row
// that also holds other controls.
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
  <Switch.Root
    v-bind="$attrs"
    :checked="props.checked"
    :disabled="props.disabled"
    @checked-change="emit('update:checked', $event.checked)"
  >
    <Switch.Label v-if="$slots.default" :class="props.labelClass">
      <slot />
    </Switch.Label>
    <Switch.Control class="switch">
      <Switch.Thumb class="switch__thumb" />
    </Switch.Control>
    <Switch.HiddenInput />
  </Switch.Root>
</template>
