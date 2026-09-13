<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from '../i18n'
import { useAccountsStore } from '../stores/accounts'
import type { EmailSearchField, EmailSearchFilter } from '../types/email'

type SearchTag = EmailSearchFilter & { locked?: boolean }
const DEFAULT_SEARCH_FIELD: EmailSearchField = 'content'

const { t } = useI18n()
const accountsStore = useAccountsStore()
const input = ref('')
const inputRef = ref<HTMLInputElement | null>(null)
const focused = ref(false)
const suggestionIndex = ref(-1)
const tags = ref<SearchTag[]>([])

const fieldLabels = computed<Record<EmailSearchField, string>>(() => ({
  from: t.value['search.from'],
  to: t.value['search.to'],
  content: t.value['search.content'],
}))

const defaultTo = computed(() => accountsStore.selectedAccount?.Email?.trim() || '')
const displayedTags = computed<SearchTag[]>(() => {
  const defaultTag = defaultTo.value
    ? [{ field: 'to' as const, value: defaultTo.value, locked: true }]
    : []
  return [...defaultTag, ...tags.value]
})

const parsedInput = computed(() => {
  const match = input.value.match(/^\s*(from|to|content)\s*:\s*(.*)$/i)
  // Unknown or missing prefixes are plain content searches by default.
  if (!match) return { field: DEFAULT_SEARCH_FIELD, value: input.value.trim() }
  return { field: match[1].toLowerCase() as EmailSearchField, value: match[2] }
})

const accountSuggestions = computed(() => {
  const { field, value } = parsedInput.value
  if ((field !== 'from' && field !== 'to') || input.value.trim() === '') return []
  const needle = value.trim().toLowerCase()
  return accountsStore.accounts
    .filter((account) => {
      if (!needle) return true
      return [account.Email, account.Name, account.IMAPUser].some((part) =>
        String(part || '')
          .toLowerCase()
          .includes(needle),
      )
    })
    .slice(0, 6)
})

const fieldSuggestions = computed(() => {
  if (/^\s*(from|to|content)\s*:/i.test(input.value)) return []
  const needle = input.value.trim().toLowerCase()
  const fields = (Object.keys(fieldLabels.value) as EmailSearchField[]).filter((field) => {
    if (!needle) return true
    return field.startsWith(needle) || fieldLabels.value[field].toLowerCase().startsWith(needle)
  })
  return fields.map((field) => ({
    field,
    label: `${field[0].toUpperCase()}${field.slice(1)}:`,
  }))
})

const showSuggestions = computed(
  () => focused.value && (accountSuggestions.value.length > 0 || fieldSuggestions.value.length > 0),
)

function scopeAccountId() {
  return accountsStore.isSmartFolder ? '' : accountsStore.selectedAccount?.ID || ''
}

function effectiveFilters() {
  return displayedTags.value.map(({ field, value }) => ({ field, value })) as EmailSearchFilter[]
}

async function runSearch() {
  if (tags.value.length === 0) {
    accountsStore.clearSearch()
    return
  }
  await accountsStore.searchEmails(effectiveFilters(), scopeAccountId())
}

async function addTag(value = parsedInput.value.value) {
  const field = parsedInput.value.field
  const normalized = value.trim()
  if (!normalized) return

  if (field === 'from') {
    tags.value = tags.value.filter((tag) => tag.field !== 'from')
  }
  if (field === 'to' && tags.value.some((tag) => tag.field === 'to' && tag.value === normalized)) {
    input.value = ''
    return
  }
  tags.value.push({ field, value: normalized })
  input.value = ''
  suggestionIndex.value = -1
  await runSearch()
}

function fillSuggestion(email: string) {
  input.value = `${parsedInput.value.field}: ${email}`
  suggestionIndex.value = -1
  void nextTick(() => inputRef.value?.focus())
}

function chooseField(field: EmailSearchField) {
  // Keep the query syntax stable across locales; labels remain localized in
  // the suggestion menu and generated tags.
  input.value = `${field[0].toUpperCase()}${field.slice(1)}: `
  suggestionIndex.value = -1
  void nextTick(() => inputRef.value?.focus())
}

function removeTag(index: number) {
  if (displayedTags.value[index]?.locked) return
  const removableIndex = index - (defaultTo.value ? 1 : 0)
  if (removableIndex >= 0) {
    tags.value.splice(removableIndex, 1)
    void runSearch()
  }
}

function onKeydown(event: KeyboardEvent) {
  const suggestions = accountSuggestions.value
  const fields = fieldSuggestions.value
  if (event.key === 'ArrowDown' && (suggestions.length > 0 || fields.length > 0)) {
    event.preventDefault()
    const count = suggestions.length || fields.length
    suggestionIndex.value = Math.min(suggestionIndex.value + 1, count - 1)
    return
  }
  if (event.key === 'ArrowUp' && (suggestions.length > 0 || fields.length > 0)) {
    event.preventDefault()
    suggestionIndex.value = Math.max(suggestionIndex.value - 1, 0)
    return
  }
  if (event.key === 'Escape') {
    focused.value = false
    return
  }
  if (event.key === 'Backspace' && input.value === '' && tags.value.length > 0) {
    tags.value.pop()
    void runSearch()
    return
  }
  if (event.key !== 'Enter' && event.key !== 'Tab') return
  if (event.key === 'Tab' && suggestions.length === 0 && fields.length === 0) return

  event.preventDefault()
  if (fields.length > 0) {
    if (suggestionIndex.value >= 0 && fields[suggestionIndex.value]) {
      chooseField(fields[suggestionIndex.value].field)
    } else if (fields.length === 1) {
      chooseField(fields[0].field)
    }
    return
  }
  const exactSuggestion = suggestions.some(
    (account) => account.Email.toLowerCase() === parsedInput.value.value.trim().toLowerCase(),
  )
  if (suggestions.length > 0 && suggestionIndex.value < 0 && !exactSuggestion) {
    fillSuggestion(suggestions[0].Email)
    return
  }
  if (suggestionIndex.value >= 0 && suggestions[suggestionIndex.value]) {
    fillSuggestion(suggestions[suggestionIndex.value].Email)
    return
  }
  void addTag()
}

watch(
  () => [accountsStore.selectedAccount?.ID, accountsStore.isSmartFolder],
  () => {
    if (tags.value.length > 0) void runSearch()
  },
)
</script>

<template>
  <div class="search-box" :class="{ focused }">
    <div class="search-tags">
      <span
        v-for="(tag, index) in displayedTags"
        :key="`${tag.field}-${tag.value}-${index}`"
        class="search-tag"
        :class="{ locked: tag.locked }"
      >
        <strong>{{ fieldLabels[tag.field] }}:</strong>
        <span>{{ tag.value }}</span>
        <button
          v-if="!tag.locked"
          type="button"
          class="tag-remove"
          :aria-label="t['search.remove']"
          @click="removeTag(index)"
        >
          ×
        </button>
      </span>
      <input
        ref="inputRef"
        v-model="input"
        class="search-input"
        type="search"
        :placeholder="displayedTags.length ? '' : t['search.placeholder']"
        @focus="focused = true"
        @blur="focused = false"
        @keydown="onKeydown"
      />
    </div>

    <div v-if="showSuggestions" class="search-suggestions">
      <button
        v-for="(suggestion, index) in accountSuggestions"
        :key="suggestion.ID"
        type="button"
        class="search-suggestion"
        :class="{ highlighted: suggestionIndex === index }"
        @mousedown.prevent="fillSuggestion(suggestion.Email)"
      >
        <span>{{ suggestion.Email }}</span>
        <small>{{ suggestion.Name || suggestion.Provider }}</small>
      </button>
      <button
        v-for="suggestion in fieldSuggestions"
        :key="suggestion.field"
        type="button"
        class="search-suggestion field-suggestion"
        @mousedown.prevent="chooseField(suggestion.field)"
      >
        <strong>{{ suggestion.label }}</strong>
        <small>{{ t[`search.${suggestion.field}`] }}</small>
      </button>
    </div>
  </div>
</template>

<style scoped>
.search-box {
  position: relative;
  width: min(620px, 100%);
}
.search-tags {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  min-height: 28px;
  padding: 2px 8px;
  background: var(--action-color, var(--n-action-color));
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
}
.search-box.focused .search-tags {
  border-color: var(--n-primary-color);
}
.search-tag {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  max-width: 260px;
  padding: 3px 5px 3px 7px;
  color: var(--n-text-color);
  background: var(--n-sidebar-active);
  border-radius: 4px;
  font-size: 12px;
  white-space: nowrap;
}
.search-tag.locked {
  background: var(--n-action-color);
  border: 1px dashed var(--n-border-color);
}
.search-tag > span {
  overflow: hidden;
  text-overflow: ellipsis;
}
.tag-remove {
  padding: 0 2px;
  color: inherit;
  background: transparent;
  border: none;
  cursor: pointer;
  opacity: 0.65;
}
.tag-remove:hover {
  opacity: 1;
}
.search-input {
  flex: 1;
  min-width: 100px;
  height: 24px;
  padding: 0 2px;
  color: var(--n-text-color);
  background: transparent;
  border: none;
  outline: none;
  font-size: 13px;
}
.search-suggestions {
  position: absolute;
  z-index: 20;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  overflow: hidden;
  padding: 4px;
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
  box-shadow: 0 8px 20px rgb(0 0 0 / 18%);
}
.search-suggestion {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  min-height: 30px;
  padding: 5px 8px;
  color: var(--n-text-color);
  text-align: left;
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}
.search-suggestion:hover,
.search-suggestion.highlighted {
  background: var(--n-sidebar-hover);
}
.search-suggestion small {
  opacity: 0.55;
}
.field-suggestion {
  justify-content: flex-start;
}
</style>
