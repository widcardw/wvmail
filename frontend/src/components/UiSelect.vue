<script setup lang="ts">
import { computed } from 'vue'
import { Select, createListCollection } from '@ark-ui/vue/select'

interface Option {
  label: string
  value: string
  disabled?: boolean
}

const props = withDefaults(
  defineProps<{
    options: Option[]
    modelValue?: string | null
    placeholder?: string
    disabled?: boolean
    width?: string
  }>(),
  { modelValue: null, placeholder: 'Select…', disabled: false, width: '220px' },
)

const emit = defineEmits<{ (e: 'update:modelValue', value: string): void }>()

const collection = computed(() =>
  createListCollection<Option>({
    items: props.options,
    itemToValue: (it) => it.value,
    itemToString: (it) => it.label,
    isItemDisabled: (it) => !!it.disabled,
  }),
)

const currentLabel = computed(
  () => props.options.find((o) => o.value === props.modelValue)?.label ?? props.placeholder,
)

// ark select handles a single selection as an array of one value
const model = computed(() => (props.modelValue ? [props.modelValue] : []))

function onValueChange(details: { value: string[] }) {
  const next = details.value?.[0]
  if (next != null) emit('update:modelValue', next)
}
</script>

<template>
  <Select.Root
    :collection="collection"
    :value="model"
    :disabled="disabled"
    @value-change="onValueChange"
  >
    <Select.Trigger class="u-trigger" type="button" :style="{ width: props.width }">
      <span class="u-value">{{ currentLabel }}</span>
      <i class="i-ri:arrow-down-s-line u-chev" aria-hidden="true" />
    </Select.Trigger>

    <Teleport to="body">
      <Select.Positioner>
        <Select.Content class="u-menu">
          <Select.Item
            v-for="opt in options"
            :key="opt.value"
            :item="opt"
            :class="opt.value === modelValue ? 'u-item selected' : 'u-item'"
          >
            <Select.ItemText>{{ opt.label }}</Select.ItemText>
            <span v-if="opt.value === modelValue" class="u-check">✓</span>
          </Select.Item>
        </Select.Content>
      </Select.Positioner>
    </Teleport>

    <Select.HiddenSelect />
  </Select.Root>
</template>

<style scoped>
.u-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 220px;
  height: 30px;
  padding: 0 8px;
  font-size: 13px;
  color: var(--n-text-color);
  background: var(--n-action-color);
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  cursor: pointer;
  user-select: none;
  text-align: left;
  font-family: inherit;
}
.u-trigger:hover {
  border-color: var(--n-primary-color);
}
.u-trigger[data-disabled] {
  opacity: 0.5;
  cursor: not-allowed;
}
.u-value {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.u-chev {
  font-size: 16px;
  opacity: 0.7;
  flex-shrink: 0;
}

.u-menu {
  z-index: 1100;
  min-width: 200px;
  max-height: 260px;
  overflow-y: auto;
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.16);
  padding: 4px;
}
.u-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 9px;
  font-size: 13px;
  border-radius: 4px;
  cursor: pointer;
  color: var(--n-text-color);
}
.u-item:hover,
.u-item[data-highlighted] {
  background: var(--n-sidebar-hover);
}
.u-item.selected {
  background: var(--n-sidebar-active);
}
.u-check {
  color: var(--n-primary-color);
}
</style>
