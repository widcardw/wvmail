<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from '~/i18n'
import { useMailSearch } from '~/composables/use-mail-search'
import type { EmailSearchField } from '~/types/email'

// The widget: the input element, focus, and keyboard navigation. The query syntax
// and the tag state machine live in composables/use-mail-search.ts; the rules behind
// them in composables/search-query.ts.
const { t } = useI18n()

const inputRef = ref<HTMLInputElement | null>(null)
const focused = ref(false)

const {
  input,
  displayedTags,
  parsedInput,
  fieldLabels,
  suggestionAccounts,
  fieldSuggestions,
  activeSuggestionIndex,
  addTag,
  removeTag,
  removeLastTag,
  chooseField: chooseFieldAndFocus,
  fillSuggestion: fillSuggestionAndFocus,
  isCompleteEmail,
} = useMailSearch({
  focusInput: () => inputRef.value?.focus(),
})

const showSuggestions = computed(
  () => focused.value && (suggestionAccounts.value.length > 0 || fieldSuggestions.value.length > 0),
)

function chooseField(field: EmailSearchField) {
  chooseFieldAndFocus(field)
}

function fillSuggestion(email: string) {
  fillSuggestionAndFocus(email)
}

function onKeydown(event: KeyboardEvent) {
  const suggestions = suggestionAccounts.value
  const fields = fieldSuggestions.value

  if (event.key === 'ArrowDown' && (suggestions.length > 0 || fields.length > 0)) {
    event.preventDefault()
    const count = suggestions.length || fields.length
    activeSuggestionIndex.value = Math.min(activeSuggestionIndex.value + 1, count - 1)
    return
  }
  if (event.key === 'ArrowUp' && (suggestions.length > 0 || fields.length > 0)) {
    event.preventDefault()
    activeSuggestionIndex.value = Math.max(activeSuggestionIndex.value - 1, 0)
    return
  }
  if (event.key === 'Escape') {
    focused.value = false
    return
  }
  if ((event.key === 'Backspace' || event.key === 'Delete') && input.value === '') {
    event.preventDefault()
    removeLastTag()
    return
  }
  if (event.key !== 'Enter' && event.key !== 'Tab') return
  if (event.key === 'Tab' && suggestions.length === 0 && fields.length === 0) return

  event.preventDefault()
  // A field name wins over an address: the user is still typing the prefix.
  if (fields.length > 0) {
    if (activeSuggestionIndex.value >= 0 && fields[activeSuggestionIndex.value]) {
      chooseField(fields[activeSuggestionIndex.value].field)
    } else if (fields.length === 1) {
      chooseField(fields[0].field)
    }
    return
  }
  const typedValue = parsedInput.value.value.trim()
  // A complete address in a from:/to: query is committed as-is; anything else falls
  // through to the highlighted (or first) suggestion.
  if (
    (parsedInput.value.field === 'from' || parsedInput.value.field === 'to') &&
    isCompleteEmail(typedValue)
  ) {
    void addTag(typedValue)
    return
  }
  const exactSuggestion = suggestions.some(
    (account) => account.Email.toLowerCase() === typedValue.toLowerCase(),
  )
  if (suggestions.length > 0 && activeSuggestionIndex.value < 0 && !exactSuggestion) {
    fillSuggestion(suggestions[0].Email)
    return
  }
  if (activeSuggestionIndex.value >= 0 && suggestions[activeSuggestionIndex.value]) {
    fillSuggestion(suggestions[activeSuggestionIndex.value].Email)
    return
  }
  void addTag()
}
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
          type="button"
          class="tag-remove"
          :aria-label="t('search.remove')"
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
        :placeholder="displayedTags.length ? '' : t('search.placeholder')"
        @focus="focused = true"
        @blur="focused = false"
        @keydown="onKeydown"
      />
    </div>

    <div v-if="showSuggestions" class="search-suggestions">
      <button
        v-for="(suggestion, index) in suggestionAccounts"
        :key="suggestion.ID"
        type="button"
        class="search-suggestion"
        :class="{ highlighted: activeSuggestionIndex === index }"
        @mousedown.prevent="addTag(suggestion.Email)"
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
        <small>{{ t(`search.${suggestion.field}`) }}</small>
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
  min-height: 28px;
  padding: 4px 8px;
  color: var(--n-text-color);
  text-align: left;
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 12px;
  line-height: 1.35;
}
.search-suggestion:hover,
.search-suggestion.highlighted {
  background: var(--n-sidebar-hover);
}
.search-suggestion small {
  font-size: 11px;
  opacity: 0.55;
}
.field-suggestion {
  justify-content: flex-start;
}
</style>
