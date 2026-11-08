<script setup lang="ts">
import { ref } from 'vue'
import { Popover } from '@ark-ui/vue/popover'
import { useI18n } from '~/i18n'
import { useTagsStore } from '~/stores/tags'
import UiCheckbox from '~/components/ui/UiCheckbox.vue'
import UiScrollArea from '~/components/ui/UiScrollArea.vue'

const props = withDefaults(
  defineProps<{
    selected: string[]
    compact?: boolean
  }>(),
  { compact: false },
)

const emit = defineEmits<{ change: [ids: string[]] }>()

const { t } = useI18n()
const tagsStore = useTagsStore()
const open = ref(false)

function setSelected(ids: string[]) {
  emit('change', [...new Set(ids)])
}

function toggle(id: string, checked: boolean) {
  const next = checked ? [...props.selected, id] : props.selected.filter((value) => value !== id)
  setSelected(next)
}

function selectAll() {
  setSelected(tagsStore.enabledTags.map((tag) => tag.id))
}

function clearAll() {
  setSelected([])
}
</script>

<template>
  <Popover.Root
    v-model:open="open"
    :positioning="{ placement: 'bottom-end', gutter: 6 }"
    lazy-mount
    unmount-on-exit
  >
    <Popover.Trigger as-child>
      <button
        type="button"
        class="tag-filter-trigger"
        :class="{ 'tag-filter-trigger--compact': compact }"
        :title="t('tags.filter')"
        :aria-label="
          selected.length ? t('tags.selectedTags', { count: selected.length }) : t('tags.filter')
        "
        :data-active="selected.length > 0"
      >
        <i :class="compact ? 'i-ri:filter-3-line' : 'i-ri:price-tag-3-line'" aria-hidden="true" />
        <template v-if="!compact">
          <span>
            {{
              selected.length
                ? t('tags.selectedTags', { count: selected.length })
                : t('tags.selectTags')
            }}
          </span>
          <i class="i-ri:arrow-down-s-line" aria-hidden="true" />
        </template>
        <span v-else-if="selected.length" class="tag-filter-count">{{ selected.length }}</span>
      </button>
    </Popover.Trigger>
    <Teleport to="body">
      <Popover.Positioner class="tag-filter-positioner">
        <Popover.Content class="tag-filter-content">
          <header class="tag-filter-head">
            <strong>{{ t('tags.selectTags') }}</strong>
            <div class="tag-filter-actions">
              <button type="button" @click="selectAll">{{ t('tags.allTags') }}</button>
              <button type="button" @click="clearAll">{{ t('tags.clearTags') }}</button>
            </div>
          </header>
          <div v-if="tagsStore.enabledTags.length === 0" class="tag-filter-empty">
            {{ t('tags.noTagsYet') }}
          </div>
          <UiScrollArea v-else class="tag-filter-scroll">
            <div class="tag-filter-list">
              <UiCheckbox
                v-for="tag in tagsStore.enabledTags"
                :key="tag.id"
                class="tag-filter-row"
                label-class="tag-filter-label"
                :checked="selected.includes(tag.id)"
                @update:checked="toggle(tag.id, $event)"
              >
                <span class="tag-filter-dot" :style="{ background: tag.color }" />
                <span>{{ tag.label }}</span>
              </UiCheckbox>
            </div>
          </UiScrollArea>
        </Popover.Content>
      </Popover.Positioner>
    </Teleport>
  </Popover.Root>
</template>

<style scoped>
.tag-filter-trigger {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  max-width: 190px;
  height: 26px;
  padding: 0 8px;
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  color: var(--n-text-color);
  background: var(--n-action-color);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}
.tag-filter-trigger:hover,
.tag-filter-trigger[data-state='open'] {
  border-color: var(--n-primary-color);
}
.tag-filter-trigger span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tag-filter-trigger i {
  flex: none;
  font-size: 14px;
}
.tag-filter-trigger--compact {
  justify-content: center;
  width: 26px;
  height: 26px;
  padding: 0;
  color: var(--n-text-color);
  background: transparent;
  border-color: transparent;
}
.tag-filter-trigger--compact:hover,
.tag-filter-trigger--compact[data-state='open'] {
  background: var(--n-sidebar-hover);
  border-color: transparent;
}
.tag-filter-trigger--compact[data-active='true'] {
  color: var(--n-primary-color);
  background: color-mix(in srgb, var(--n-primary-color) 12%, transparent);
}
.tag-filter-count {
  position: absolute;
  top: -4px;
  right: -4px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 14px;
  height: 14px;
  padding: 0 3px;
  color: #fff;
  background: var(--n-primary-color);
  border: 1px solid var(--n-body-color);
  border-radius: 999px;
  font-size: 9px;
  line-height: 1;
}
.tag-filter-positioner {
  z-index: 1100;
}
.tag-filter-content {
  position: relative;
  z-index: 1;
  width: 260px;
  padding: 8px;
  overflow: hidden;
  color: var(--n-text-color);
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
  box-shadow:
    0 12px 32px rgb(0 0 0 / 26%),
    0 2px 8px rgb(0 0 0 / 16%);
}
.tag-filter-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 2px 4px 8px;
  font-size: 12px;
}
.tag-filter-actions {
  display: flex;
  gap: 8px;
}
.tag-filter-actions button {
  padding: 0;
  border: none;
  color: var(--n-primary-color);
  background: transparent;
  font: inherit;
  font-size: 11px;
  cursor: pointer;
}
.tag-filter-scroll {
  max-height: 280px;
}
.tag-filter-list {
  display: flex;
  flex-direction: column;
  gap: 1px;
}
.tag-filter-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 6px;
  border-radius: 5px;
  font-size: 12px;
}
.tag-filter-row:hover {
  background: var(--n-sidebar-hover);
}
.tag-filter-label {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
}
.tag-filter-dot {
  width: 8px;
  height: 8px;
  flex: none;
  border-radius: 999px;
}
.tag-filter-empty {
  padding: 12px 6px;
  color: var(--color-text-3);
  font-size: 12px;
}
</style>
