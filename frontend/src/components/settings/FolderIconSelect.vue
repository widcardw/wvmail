<script setup lang="ts">
import { computed } from 'vue'
import { Select, createListCollection } from '@ark-ui/vue/select'
import { useI18n } from '~/i18n'
import {
  AUTO_FOLDER_ICON,
  FOLDER_ICON_CHOICES,
  FOLDER_ICON_GROUPS,
  type FolderIconChoice,
} from './folder-icons'

const props = withDefaults(
  defineProps<{
    modelValue?: string | null
    disabled?: boolean
    width?: string
    /**
     * The icon the folder resolves to automatically. It is shown on the trigger
     * (and on the "automatic" menu row) so the row previews the real sidebar
     * icon instead of a generic wand whenever no custom icon is set.
     */
    autoIcon?: string
  }>(),
  { modelValue: '', disabled: false, width: '210px', autoIcon: '' },
)

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const { t } = useI18n()

const autoIconClass = computed(() => `i-${props.autoIcon || 'ri:magic-line'}`)

// One collection holds every choice (the first entry is "automatic"); the menu
// renders it grouped so 120+ icons stay scannable.
const options = computed(() =>
  createListCollection<FolderIconChoice>({
    items: FOLDER_ICON_CHOICES,
    itemToValue: (icon) => icon.value,
    itemToString: (icon) => t.value[icon.labelKey],
  }),
)

const selectedValue = computed(() => props.modelValue ?? '')
const selectedLabel = computed(() => {
  const found = FOLDER_ICON_CHOICES.find((icon) => icon.value === selectedValue.value)
  return found ? t.value[found.labelKey] : selectedValue.value || t.value[AUTO_FOLDER_ICON.labelKey]
})
const model = computed(() => [selectedValue.value])

function onValueChange(details: { value: string[] }) {
  const next = details.value?.[0]
  // The empty string is a valid value here: it means "automatic".
  if (next != null) emit('update:modelValue', next)
}
</script>

<template>
  <Select.Root
    :collection="options"
    :value="model"
    :disabled="props.disabled"
    @value-change="onValueChange"
  >
    <Select.Trigger class="icon-select" type="button" :style="{ width: props.width }">
      <i
        v-if="selectedValue"
        :class="`i-${selectedValue}`"
        class="icon-select__current"
        aria-hidden="true"
      />
      <i v-else :class="autoIconClass" class="icon-select__current icon-select__auto" aria-hidden="true" />
      <span class="icon-select__label">{{ selectedLabel }}</span>
      <i class="i-ri:arrow-down-s-line icon-select__chev" aria-hidden="true" />
    </Select.Trigger>
    <Teleport to="body">
      <Select.Positioner>
        <Select.Content class="icon-select__menu">
          <Select.Item
            :item="AUTO_FOLDER_ICON"
            class="icon-select__auto-item"
            :class="{ 'icon-select__auto-item--selected': selectedValue === '' }"
          >
            <i :class="autoIconClass" aria-hidden="true" />
            <Select.ItemText>{{ t['settings.accounts.folder_icons.auto'] }}</Select.ItemText>
            <span v-if="selectedValue === ''" class="icon-select__check">✓</span>
          </Select.Item>

          <div v-for="group in FOLDER_ICON_GROUPS" :key="group.labelKey" class="icon-select__group">
            <div class="icon-select__group-label">{{ t[group.labelKey] }}</div>
            <div class="icon-select__grid">
              <Select.Item
                v-for="icon in group.icons"
                :key="icon.value"
                :item="icon"
                class="icon-select__cell"
                :class="{ 'icon-select__cell--selected': icon.value === selectedValue }"
                :title="t[icon.labelKey]"
                :aria-label="t[icon.labelKey]"
              >
                <i :class="`i-${icon.value}`" aria-hidden="true" />
                <Select.ItemText class="icon-select__sr">{{ t[icon.labelKey] }}</Select.ItemText>
              </Select.Item>
            </div>
          </div>
        </Select.Content>
      </Select.Positioner>
    </Teleport>
    <Select.HiddenSelect />
  </Select.Root>
</template>

<style scoped>
.icon-select,
.icon-select__menu {
  z-index: 1100;
}
.icon-select {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  width: 210px;
  height: 30px;
  padding: 0 8px;
  color: var(--n-text-color);
  font-size: 13px;
  background: var(--n-action-color);
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  cursor: pointer;
  font-family: inherit;
  text-align: left;
  user-select: none;
}
.icon-select:hover:not([data-disabled]) {
  border-color: var(--n-primary-color);
}
.icon-select[data-disabled] {
  color: var(--color-text-3);
  background: var(--color-hover);
  cursor: not-allowed;
}
.icon-select__current {
  flex: 0 0 16px;
  color: var(--n-primary-color);
  font-size: 15px;
}
.icon-select__auto {
  color: var(--color-text-3);
}
.icon-select__label {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.icon-select__chev {
  flex: 0 0 16px;
  font-size: 16px;
  opacity: 0.7;
}
.icon-select__menu {
  width: 318px;
  max-height: 340px;
  overflow-y: auto;
  padding: 4px;
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgb(0 0 0 / 16%);
}
/* "Automatic" sits above the grid as a plain row: it is the default and reads
   better as a sentence than as another tile. */
.icon-select__auto-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  color: var(--n-text-color);
  font-size: 13px;
  border-radius: 4px;
  cursor: pointer;
}
.icon-select__auto-item:hover,
.icon-select__auto-item[data-highlighted] {
  background: var(--n-sidebar-hover);
}
.icon-select__auto-item--selected {
  background: var(--n-sidebar-active);
}
.icon-select__auto-item > i {
  flex: 0 0 16px;
  color: var(--color-text-3);
  font-size: 15px;
}
.icon-select__group {
  margin-top: 4px;
}
.icon-select__group-label {
  position: sticky;
  top: -4px;
  z-index: 1;
  padding: 6px 6px 4px;
  color: var(--color-text-3);
  font-size: 11px;
  font-weight: 600;
  background: var(--color-surface);
}
.icon-select__grid {
  display: grid;
  grid-template-columns: repeat(9, 1fr);
  gap: 3px;
  padding: 0 2px 2px;
}
.icon-select__cell {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 32px;
  color: var(--n-text-color);
  font-size: 17px;
  border: 1px solid transparent;
  border-radius: 5px;
  cursor: pointer;
}
.icon-select__cell:hover,
.icon-select__cell[data-highlighted] {
  background: var(--n-sidebar-hover);
}
.icon-select__cell--selected {
  color: var(--n-primary-color);
  background: var(--n-sidebar-active);
  border-color: var(--n-primary-color);
}
/* ItemText is required by Ark for typeahead/a11y; the cell shows only the icon. */
.icon-select__sr {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  white-space: nowrap;
  clip-path: inset(50%);
}
.icon-select__check {
  margin-left: auto;
  color: var(--n-primary-color);
}
</style>
