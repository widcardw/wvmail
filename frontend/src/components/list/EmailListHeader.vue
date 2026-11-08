<script setup lang="ts">
import { Editable } from '@ark-ui/vue/editable'
import { Pagination } from '@ark-ui/vue/pagination'
import { ref, watch } from 'vue'
import { useI18n } from '~/i18n'

const props = defineProps<{
  label: string
  title: string
  showRefresh: boolean
  refreshTitle?: string
  refreshing: boolean
  pageCount: number
  total: number
  pageSize: number
  page: number
}>()

const emit = defineEmits<{
  refresh: []
  pageChange: [page: number]
}>()

const { t } = useI18n()
const pageInput = ref('1')

watch(
  () => props.page,
  (page) => {
    pageInput.value = String(page)
  },
  { immediate: true },
)

function onPageChange(details: unknown) {
  const page = (details as { page?: number }).page
  if (typeof page === 'number' && page >= 1) emit('pageChange', page)
}

function onPageCommit(details: unknown) {
  const value = (details as { value?: string }).value ?? pageInput.value
  const page = Number.parseInt(value, 10)

  if (!Number.isInteger(page)) {
    pageInput.value = String(props.page)
    return
  }

  const nextPage = Math.min(Math.max(page, 1), props.pageCount)
  pageInput.value = String(nextPage)
  if (nextPage !== props.page) emit('pageChange', nextPage)
}
</script>

<template>
  <div class="list-header">
    <span class="folder-label" :title="title">{{ label }}</span>
    <slot name="filters" />
    <button
      v-if="showRefresh"
      type="button"
      class="icon-btn"
      :title="refreshTitle || t('mail.refresh.list')"
      :disabled="refreshing"
      @click="emit('refresh')"
    >
      <i class="i-ri:refresh-line" :class="{ 'refresh-spin': refreshing }" aria-hidden="true" />
    </button>
    <slot name="after-refresh" />
    <span class="spacer" />
    <Pagination.Root
      v-if="pageCount > 1"
      :count="total"
      :page-size="pageSize"
      :page="page"
      :sibling-count="1"
      class="pager-root"
      @page-change="onPageChange"
    >
      <Pagination.PrevTrigger class="pg">
        <i class="i-ri:arrow-left-s-line" />
      </Pagination.PrevTrigger>
      <Editable.Root
        v-model="pageInput"
        class="pager-editable"
        submit-mode="enter"
        :aria-label="t('mail.page')"
        @value-commit="onPageCommit"
      >
        <Editable.Area>
          <Editable.Preview />
          <Editable.Input
            type="number"
            min="1"
            :max="pageCount"
            inputmode="numeric"
            :aria-label="t('mail.page')"
          />
        </Editable.Area>
      </Editable.Root>
      <span class="pager-separator" aria-hidden="true">/</span>
      <span class="pager-total">{{ pageCount }}</span>
      <Pagination.NextTrigger class="pg">
        <i class="i-ri:arrow-right-s-line" />
      </Pagination.NextTrigger>
    </Pagination.Root>
  </div>
</template>

<style scoped>
.list-header {
  flex-shrink: 0;
  height: var(--pane-header-height);
  padding: 0 10px;
  border-bottom: 1px solid var(--n-border-color);
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}
.folder-label {
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  white-space: nowrap;
  color: var(--n-text-color);
}
.spacer {
  flex: 1;
}
.refresh-spin {
  animation: refresh-rotate 0.8s linear infinite;
}
@keyframes refresh-rotate {
  to {
    transform: rotate(360deg);
  }
}
.pager-root {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.pg {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 22px;
  height: 22px;
  padding: 0;
  border: 1px solid transparent;
  background: transparent;
  border-radius: 5px;
  cursor: pointer;
  font-size: 13px;
  color: var(--n-text-color);
}
.pg:hover {
  background: var(--n-sidebar-hover);
}
.pg[data-selected] {
  background: var(--n-primary-color);
  color: #fff;
}
.pg[data-disabled],
.pg:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
.pg i {
  font-size: 16px;
}
.pager-editable {
  display: inline-flex;
  min-width: 26px;
  height: 22px;
}
.pager-editable [data-part='area'] {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 26px;
  height: 22px;
}
.pager-editable [data-part='preview'],
.pager-editable [data-part='input'] {
  width: 26px;
  height: 22px;
  padding: 0 2px;
  border: 1px solid transparent;
  border-radius: 5px;
  color: var(--n-text-color);
  background: transparent;
  font: inherit;
  font-size: 13px;
  line-height: 20px;
  text-align: center;
}
.pager-editable [data-part='preview'] {
  cursor: text;
}
.pager-editable [data-part='preview']:hover,
.pager-editable [data-part='input']:focus {
  border-color: var(--n-primary-color);
  outline: none;
  background: var(--n-action-color);
}
.pager-editable [data-part='input']::-webkit-inner-spin-button,
.pager-editable [data-part='input']::-webkit-outer-spin-button {
  margin: 0;
  appearance: none;
}
.pager-separator,
.pager-total {
  color: var(--color-text-2);
  font-size: 13px;
  line-height: 22px;
}
</style>
