<script setup lang="ts">
import { Field } from '@ark-ui/vue/field'

/**
 * One settings row: a Field.Root with a label and a control, laid out on the
 * shared grid (label column + control column, hint on a second row).
 *
 * This replaces the 19 hand-copied `Field.Root class="row"` blocks across the
 * settings views. The hint lives on a second row of the control column, not in a
 * wrapper element: Ark renders Field.HelperText as a <span>, but the grid item
 * makes it a block on its own row.
 */
defineProps<{
  label: string
  /** When true, the hint sits on the control column's second row. */
  hint?: string
  disabled?: boolean
}>()
</script>

<template>
  <Field.Root class="field-row" :disabled="disabled">
    <Field.Label class="field-row__label">{{ label }}</Field.Label>
    <div class="field-row__control">
      <slot />
    </div>
    <Field.HelperText v-if="hint" class="field-row__hint">
      {{ hint }}
    </Field.HelperText>
  </Field.Root>
</template>

<style scoped>
.field-row {
  display: grid;
  grid-template-columns: 240px 1fr;
  align-items: center;
  column-gap: 16px;
  row-gap: 0;
  padding: 10px 18px;
}
.field-row__label {
  font-size: 13px;
  color: var(--color-text-2);
  line-height: 2.15;
}
.field-row__control {
  width: 100%;
  min-width: 0;
}
.field-row__hint {
  grid-column: 2;
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--color-text-3);
  line-height: 1.6;
}
</style>
