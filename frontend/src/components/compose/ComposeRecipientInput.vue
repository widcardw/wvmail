<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useContactsStore } from '~/stores/contacts'
import { useI18n } from '~/i18n'
import {
  completeCurrentRecipient,
  currentRecipientQuery,
  parseRecipientText,
} from '~/composables/compose'
import { matchSavedContacts, SUGGESTION_LIMIT } from '~/composables/search-query'
import type { ContactSuggestion } from '~/composables/search-query'

const props = defineProps<{
  id: string
  modelValue: string
  placeholder: string
  label: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const { t } = useI18n()
const contactsStore = useContactsStore()
const inputRef = ref<HTMLInputElement | null>(null)
const focused = ref(false)
const dismissed = ref(false)
const activeIndex = ref(-1)
let blurTimer: ReturnType<typeof setTimeout> | undefined

void contactsStore.loadContacts()

const value = computed({
  get: () => props.modelValue,
  set: (next: string) => emit('update:modelValue', next),
})

const selectedEmails = computed(
  () =>
    new Set(
      parseRecipientText(props.modelValue).map((address) => address.Address.trim().toLowerCase()),
    ),
)

const availableContacts = computed(() =>
  contactsStore.sortedContacts.filter(
    (contact) => !selectedEmails.value.has(contact.email.trim().toLowerCase()),
  ),
)

const query = computed(() => currentRecipientQuery(props.modelValue).trim())

const suggestions = computed<ContactSuggestion[]>(() => {
  if (query.value) {
    return matchSavedContacts(availableContacts.value, query.value, SUGGESTION_LIMIT)
  }
  return availableContacts.value.slice(0, SUGGESTION_LIMIT).map((contact) => ({
    ID: `saved:${contact.id}`,
    address: contact.email,
    name: contact.name,
  }))
})

const showSuggestions = computed(
  () => focused.value && !dismissed.value && suggestions.value.length > 0,
)
const listId = computed(() => `${props.id}-suggestions`)
const activeOptionId = computed(() =>
  activeIndex.value >= 0 ? `${listId.value}-${activeIndex.value}` : undefined,
)

watch(
  () => suggestions.value.length,
  (length) => {
    if (length === 0) activeIndex.value = -1
    else if (activeIndex.value >= length) activeIndex.value = length - 1
  },
)

function suggestionText(contact: ContactSuggestion): string {
  return contact.name ? `${contact.name} <${contact.address}>` : contact.address
}

function choose(contact: ContactSuggestion) {
  value.value = completeCurrentRecipient(props.modelValue, suggestionText(contact), true)
  dismissed.value = true
  activeIndex.value = -1
  void nextTick(() => inputRef.value?.focus())
}

function onFocus() {
  if (blurTimer) clearTimeout(blurTimer)
  focused.value = true
  dismissed.value = false
}

function onBlur() {
  if (blurTimer) clearTimeout(blurTimer)
  blurTimer = setTimeout(() => {
    focused.value = false
    activeIndex.value = -1
  }, 100)
}

function onInput(event: Event) {
  value.value = (event.target as HTMLInputElement).value
  dismissed.value = false
  activeIndex.value = suggestions.value.length > 0 ? 0 : -1
}

function onKeydown(event: KeyboardEvent) {
  if (event.isComposing || !showSuggestions.value) return
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    const direction = event.key === 'ArrowDown' ? 1 : -1
    const count = suggestions.value.length
    activeIndex.value = (activeIndex.value + direction + count) % count
    return
  }
  if (event.key === 'Enter' || event.key === 'Tab') {
    event.preventDefault()
    const contact = suggestions.value[activeIndex.value] ?? suggestions.value[0]
    if (contact) choose(contact)
    return
  }
  if (event.key === 'Escape') {
    event.preventDefault()
    dismissed.value = true
    activeIndex.value = -1
  }
}
</script>

<template>
  <div class="recipient-input">
    <input
      :id="id"
      ref="inputRef"
      v-model="value"
      class="recipient-input__control"
      type="text"
      autocomplete="off"
      autocapitalize="off"
      autocorrect="off"
      spellcheck="false"
      role="combobox"
      aria-autocomplete="list"
      :aria-label="label"
      :aria-expanded="showSuggestions"
      :aria-controls="listId"
      :aria-activedescendant="activeOptionId"
      :placeholder="placeholder"
      @focus="onFocus"
      @blur="onBlur"
      @input="onInput"
      @keydown="onKeydown"
    />

    <div v-if="showSuggestions" :id="listId" class="recipient-suggestions" role="listbox">
      <div class="recipient-suggestions__title">{{ t['contacts.title'] }}</div>
      <button
        v-for="(contact, index) in suggestions"
        :id="`${listId}-${index}`"
        :key="contact.ID || contact.address"
        type="button"
        role="option"
        tabindex="-1"
        class="recipient-suggestion"
        :class="{ 'recipient-suggestion--active': activeIndex === index }"
        :aria-selected="activeIndex === index"
        @mouseenter="activeIndex = index"
        @mousedown.prevent="choose(contact)"
      >
        <i class="i-ri:user-3-line" aria-hidden="true" />
        <span class="recipient-suggestion__text">
          <strong>{{ contact.name || contact.address }}</strong>
          <small v-if="contact.name">{{ contact.address }}</small>
        </span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.recipient-input {
  position: relative;
  flex: 1;
  min-width: 0;
}
.recipient-input__control {
  width: 100%;
  height: 34px;
  padding: 0;
  color: var(--n-text-color);
  background: transparent;
  border: none;
  outline: none;
  font: inherit;
  font-size: 13px;
}
.recipient-input__control::placeholder {
  color: var(--color-text-3);
}
.recipient-suggestions {
  position: absolute;
  z-index: 30;
  top: calc(100% + 5px);
  right: 0;
  left: 0;
  max-height: 280px;
  padding: 4px;
  overflow-y: auto;
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  box-shadow: 0 10px 28px rgb(0 0 0 / 14%);
}
.recipient-suggestions__title {
  padding: 5px 8px 4px;
  color: var(--color-text-3);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}
.recipient-suggestion {
  display: flex;
  align-items: center;
  gap: 9px;
  width: 100%;
  min-height: 42px;
  padding: 6px 8px;
  color: var(--n-text-color);
  text-align: left;
  background: transparent;
  border: none;
  border-radius: 5px;
  cursor: pointer;
  font: inherit;
}
.recipient-suggestion:hover,
.recipient-suggestion--active {
  background: var(--n-sidebar-hover);
}
.recipient-suggestion > i {
  flex-shrink: 0;
  color: var(--color-text-3);
  font-size: 17px;
}
.recipient-suggestion__text {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 2px;
}
.recipient-suggestion__text strong,
.recipient-suggestion__text small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.recipient-suggestion__text strong {
  font-size: 12px;
  font-weight: 600;
}
.recipient-suggestion__text small {
  color: var(--color-text-3);
  font-size: 11px;
}
</style>
