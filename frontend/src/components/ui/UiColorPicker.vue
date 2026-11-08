<script setup lang="ts">
import { computed, ref } from 'vue'
import { Popover } from '@ark-ui/vue/popover'

const props = withDefaults(
  defineProps<{
    label: string
    customLabel: string
    defaultLabel: string
    icon: string
    value?: string | null
    fallback: string
    presets?: string[]
    variant?: 'toolbar' | 'field'
  }>(),
  {
    value: null,
    variant: 'toolbar',
    presets: () => [
      '#111827',
      '#374151',
      '#dc2626',
      '#ea580c',
      '#ca8a04',
      '#16a34a',
      '#0891b2',
      '#2563eb',
      '#7c3aed',
      '#db2777',
    ],
  },
)

const emit = defineEmits<{
  select: [color: string]
  reset: []
}>()

const open = ref(false)

const normalizedValue = computed(() => {
  const value = props.value?.trim() ?? ''
  return /^#[0-9a-f]{6}$/i.test(value) ? value : props.fallback
})

function selectColor(color: string) {
  emit('select', color)
  open.value = false
}

function resetColor() {
  emit('reset')
  open.value = false
}

function onCustomColor(event: Event) {
  const input = event.target as HTMLInputElement
  if (input.value) selectColor(input.value)
}
</script>

<template>
  <Popover.Root
    v-model:open="open"
    :positioning="{ placement: 'bottom-start', gutter: 6 }"
    lazy-mount
    unmount-on-exit
  >
    <Popover.Trigger
      class="color-tb"
      :class="[`color-tb--${variant}`, { active: value }]"
      :title="label"
      :aria-label="label"
    >
      <template v-if="variant === 'toolbar'">
        <i :class="icon" aria-hidden="true" />
        <span class="color-strip" :style="{ backgroundColor: normalizedValue }" />
      </template>
      <template v-else>
        <span class="color-field-swatch" :style="{ backgroundColor: normalizedValue }" />
        <span class="color-field-value">{{ normalizedValue }}</span>
        <i class="i-ri:arrow-down-s-line color-field-chevron" aria-hidden="true" />
      </template>
    </Popover.Trigger>
    <Teleport to="body">
      <Popover.Positioner class="color-positioner">
        <Popover.Content class="color-content">
          <div class="color-grid" role="group" :aria-label="label">
            <button
              v-for="color in presets"
              :key="color"
              type="button"
              class="color-swatch"
              :class="{ selected: value?.toLowerCase() === color.toLowerCase() }"
              :style="{ backgroundColor: color }"
              :title="color"
              :aria-label="color"
              @click="selectColor(color)"
            />
          </div>
          <div class="color-actions">
            <label class="color-custom">
              <input
                type="color"
                class="color-native"
                :value="normalizedValue"
                @change="onCustomColor"
              />
              <i class="i-ri:palette-line" aria-hidden="true" />
              <span>{{ customLabel }}</span>
            </label>
            <button type="button" class="color-reset" @click="resetColor">
              {{ defaultLabel }}
            </button>
          </div>
        </Popover.Content>
      </Popover.Positioner>
    </Teleport>
  </Popover.Root>
</template>

<style scoped>
.color-tb {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  color: var(--color-text-2);
  background: transparent;
  border: 1px solid transparent;
  border-radius: 5px;
  cursor: pointer;
}
.color-tb:hover,
.color-tb[data-state='open'],
.color-tb.active {
  color: var(--n-text-color);
  background: var(--n-sidebar-hover);
}
.color-tb.active {
  color: var(--n-primary-color);
  border-color: color-mix(in srgb, var(--n-primary-color) 32%, transparent);
}
.color-tb i {
  font-size: 16px;
}
.color-strip {
  position: absolute;
  right: 6px;
  bottom: 4px;
  left: 6px;
  height: 3px;
  border-radius: 999px;
  box-shadow: inset 0 0 0 1px rgb(0 0 0 / 16%);
}
.color-tb--field {
  width: 100%;
  height: 32px;
  justify-content: flex-start;
  gap: 8px;
  padding: 0 9px;
  color: var(--n-text-color);
  background: var(--n-action-color);
  border-color: var(--n-border-color);
  font-size: 12px;
}
.color-tb--field:hover,
.color-tb--field[data-state='open'] {
  color: var(--n-text-color);
  background: var(--n-action-color);
  border-color: var(--n-primary-color);
}
.color-tb--field.active {
  color: var(--n-text-color);
  border-color: var(--n-border-color);
}
.color-tb--field[data-state='open'] {
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--n-primary-color) 18%, transparent);
}
.color-field-swatch {
  width: 18px;
  height: 18px;
  flex: none;
  border: 1px solid rgb(128 128 128 / 38%);
  border-radius: 4px;
}
.color-field-value {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}
.color-field-chevron {
  margin-left: auto;
  color: var(--color-text-3);
  font-size: 14px;
}

.color-positioner {
  /* Ark writes `--z-index` inline on the positioner. Inside a modal dialog that
     can resolve to the dialog's own layer, leaving the palette visually behind
     the card even though Ark considers it open and moves focus into it. */
  z-index: 1200 !important;
}
.color-content {
  position: relative;
  z-index: 1201;
  width: 188px;
  padding: 8px;
  color: var(--n-text-color);
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
  box-shadow: 0 8px 20px rgb(0 0 0 / 18%);
}
.color-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 6px;
}
.color-swatch {
  width: 28px;
  height: 24px;
  padding: 0;
  border: 1px solid rgb(128 128 128 / 35%);
  border-radius: 5px;
  cursor: pointer;
}
.color-swatch:hover {
  outline: 2px solid var(--n-primary-color);
  outline-offset: 1px;
}
.color-swatch.selected {
  outline: 2px solid var(--n-primary-color);
  outline-offset: 1px;
}
.color-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--n-border-color);
}
.color-custom,
.color-reset {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 26px;
  padding: 0 7px;
  color: var(--n-text-color);
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
}
.color-custom:hover,
.color-reset:hover {
  background: var(--n-sidebar-hover);
}
.color-native {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
  pointer-events: none;
}
.color-custom i {
  font-size: 15px;
}
.color-reset {
  color: var(--color-text-2);
}
</style>
