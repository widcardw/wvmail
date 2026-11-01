<script setup lang="ts">
import { ref } from 'vue'
import { Popover } from '@ark-ui/vue/popover'
import { useI18n } from '~/i18n'
import { useTagsStore } from '~/stores/tags'
import UiCheckbox from '~/components/ui/UiCheckbox.vue'

const { t, tf } = useI18n()
const tagsStore = useTagsStore()
const open = ref(false)

function toggle(id: string, checked: boolean) {
  const next = checked
    ? [...tagsStore.selectedFilterIDs, id]
    : tagsStore.selectedFilterIDs.filter((value) => value !== id)
  void tagsStore.setFilterIDs(next)
}

function selectAll() {
  void tagsStore.setFilterIDs(tagsStore.enabledTags.map((tag) => tag.id))
}

function clearAll() {
  void tagsStore.setFilterIDs([])
}
</script>

<template>
  <Popover.Root v-model:open="open" :positioning="{ placement: 'bottom-end', gutter: 6 }">
    <Popover.Trigger as-child>
      <button
        type="button"
        class="tag-filter-trigger"
        :title="t['tags.filter']"
        :aria-label="t['tags.filter']"
      >
        <i class="i-ri:price-tag-3-line" aria-hidden="true" />
        <span>
          {{
            tagsStore.selectedFilterIDs.length
              ? tf('tags.selectedTags', { count: tagsStore.selectedFilterIDs.length })
              : t['tags.selectTags']
          }}
        </span>
        <i class="i-ri:arrow-down-s-line" aria-hidden="true" />
      </button>
    </Popover.Trigger>
    <Teleport to="body">
      <Popover.Positioner class="tag-filter-positioner">
        <Popover.Content class="tag-filter-content">
          <header class="tag-filter-head">
            <strong>{{ t['tags.selectTags'] }}</strong>
            <div class="tag-filter-actions">
              <button type="button" @click="selectAll">{{ t['tags.allTags'] }}</button>
              <button type="button" @click="clearAll">{{ t['tags.clearTags'] }}</button>
            </div>
          </header>
          <div v-if="tagsStore.enabledTags.length === 0" class="tag-filter-empty">
            {{ t['tags.noTagsYet'] }}
          </div>
          <div v-else class="tag-filter-list">
            <UiCheckbox
              v-for="tag in tagsStore.enabledTags"
              :key="tag.id"
              class="tag-filter-row"
              label-class="tag-filter-label"
              :checked="tagsStore.selectedFilterIDs.includes(tag.id)"
              @update:checked="toggle(tag.id, $event)"
            >
              <span class="tag-filter-dot" :style="{ background: tag.color }" />
              <span>{{ tag.label }}</span>
            </UiCheckbox>
          </div>
        </Popover.Content>
      </Popover.Positioner>
    </Teleport>
  </Popover.Root>
</template>

<style scoped>
.tag-filter-trigger {
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
.tag-filter-positioner {
  z-index: 1100;
}
.tag-filter-content {
  width: 260px;
  padding: 8px;
  color: var(--n-text-color);
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
  box-shadow: 0 10px 28px rgb(0 0 0 / 20%);
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
.tag-filter-list {
  display: flex;
  max-height: 280px;
  flex-direction: column;
  gap: 1px;
  overflow-y: auto;
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
