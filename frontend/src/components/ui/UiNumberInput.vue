<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { NumberInput } from '@ark-ui/vue/number-input'

// Thin wrapper around Ark's NumberInput: one visual definition for every numeric
// field plus the stepper buttons, clamping and keyboard handling Ark brings.
//
// Ark's NumberInput is *controlled*, which is why this wrapper is too: it takes a
// number and emits `value`. The call sites therefore read `@value="x = $event"`
// instead of reading `.value` off a DOM event (see `onNumber` in LLMEditorDialog).
//
// The visible box is `NumberInput.Control` (class `.num`), so the caller's `class`
// — normally a width / flex basis — lands on that box and only the remaining
// `$attrs` reach the input. The steppers sit in flow after the input instead of
// being absolutely positioned: anchored to a control that can be wider than the
// input (`.input--sm`, `.numsel`) they used to float to the edge of the row.
defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    value: number
    min?: number
    max?: number
    step?: number
    disabled?: boolean
  }>(),
  { min: undefined, max: undefined, step: undefined, disabled: false },
)

const emit = defineEmits<{ value: [value: number] }>()

const attrs = useAttrs()
const rootClass = computed(() => attrs.class as string | undefined)
const inputAttrs = computed(() => {
  const { class: _class, ...rest } = attrs
  return rest
})
</script>

<template>
  <NumberInput.Root
    :model-value="String(props.value)"
    :min="props.min"
    :max="props.max"
    :step="props.step"
    :disabled="props.disabled"
    @value-change="emit('value', $event.valueAsNumber)"
  >
    <NumberInput.Control class="num" :class="rootClass">
      <NumberInput.Input v-bind="inputAttrs" class="num__input" />
      <div class="num__steppers">
        <NumberInput.IncrementTrigger type="button" class="num__btn" aria-label="+">
          <i class="i-ri:arrow-up-s-line" aria-hidden="true" />
        </NumberInput.IncrementTrigger>
        <NumberInput.DecrementTrigger type="button" class="num__btn" aria-label="-">
          <i class="i-ri:arrow-down-s-line" aria-hidden="true" />
        </NumberInput.DecrementTrigger>
      </div>
    </NumberInput.Control>
  </NumberInput.Root>
</template>
