<script setup lang="ts">
import { PasswordInput } from '@ark-ui/vue/password-input'
import { computed, useAttrs } from 'vue'

// Thin wrapper around Ark's PasswordInput: one visual definition for every secret
// field plus the show/hide trigger the plain <input type="password"> never had.
// It keeps a native-like value/input contract, so call sites still read the same
// way as `strVal($event)` / `onInput(key, $event)` expect.
defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{ value: string; placeholder?: string; disabled?: boolean; width?: string }>(),
  { placeholder: '', disabled: false, width: '220px' },
)

const emit = defineEmits<{ input: [event: Event] }>()
const attrs = useAttrs()
const rootClass = computed(() => attrs.class as string | undefined)
</script>

<template>
  <PasswordInput.Root :disabled="props.disabled" :width="width">
    <PasswordInput.Control class="pw">
      <PasswordInput.Input
        v-bind="$attrs"
        class="pw__input"
        :value="props.value"
        :placeholder="props.placeholder"
        @input="emit('input', $event)"
      />
      <PasswordInput.VisibilityTrigger
        type="button"
        class="pw__toggle"
        :aria-label="props.disabled ? '' : 'toggle'"
      >
        <!-- Ark reflects the trigger state on data-state; swap the glyph in CSS so
             no extra JS state is needed here. -->
        <i class="i-ri:eye-line pw__eye" aria-hidden="true" />
        <i class="i-ri:eye-off-line pw__eye-off" aria-hidden="true" />
      </PasswordInput.VisibilityTrigger>
    </PasswordInput.Control>
  </PasswordInput.Root>
</template>
