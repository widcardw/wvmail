<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Field } from '@ark-ui/vue/field'
import { useI18n } from '~/i18n'
import { COMPLETE_EMAIL_PATTERN } from '~/composables/search-query'
import { useContactsStore } from '~/stores/contacts'
import { useFeedbackStore } from '~/stores/feedback'
import { usePreferencesStore } from '~/stores/preferences'
import UiDialog from '~/components/ui/UiDialog.vue'
import UiDialogHeader from '~/components/ui/UiDialogHeader.vue'
import ConfirmDialog from '~/components/ui/ConfirmDialog.vue'
import UiScrollArea from '~/components/ui/UiScrollArea.vue'
import UiSwitch from '~/components/ui/UiSwitch.vue'
import type { Contact } from '~/types/contact'

const props = withDefaults(
  defineProps<{
    /** Embedded in Settings, whose page already owns the scroll container. */
    embedded?: boolean
  }>(),
  { embedded: false },
)

const { t, tf } = useI18n()
const contactsStore = useContactsStore()
const feedback = useFeedbackStore()
const preferences = usePreferencesStore()

const filter = ref('')
const dialogOpen = ref(false)
const editing = ref<Contact | null>(null)
const formName = ref('')
const formEmail = ref('')
const saving = ref(false)
const deleteTarget = ref<Contact | null>(null)
const deleting = ref(false)

const visibleContacts = computed(() => {
  const needle = filter.value.trim().toLowerCase()
  if (!needle) return contactsStore.sortedContacts
  return contactsStore.sortedContacts.filter((contact) =>
    [contact.name, contact.email].some((part) => part.toLowerCase().includes(needle)),
  )
})

const emailValid = computed(() => COMPLETE_EMAIL_PATTERN.test(formEmail.value.trim()))

function displayName(contact: Contact): string {
  return contact.name.trim() || contact.email.split('@')[0] || t.value['contacts.unnamed']
}

function initials(contact: Contact): string {
  const source = displayName(contact).trim()
  return Array.from(source)[0]?.toUpperCase() || '?'
}

function openAdd() {
  editing.value = null
  formName.value = ''
  formEmail.value = ''
  dialogOpen.value = true
}

function onAutoCollectChange(checked: boolean) {
  void preferences.setAutoCollectContacts(checked)
}

function openEdit(contact: Contact) {
  editing.value = contact
  formName.value = contact.name
  formEmail.value = contact.email
  dialogOpen.value = true
}

async function save() {
  if (!emailValid.value || saving.value) return
  saving.value = true
  const saved = await contactsStore.saveContact({
    id: editing.value?.id || '',
    name: formName.value.trim(),
    email: formEmail.value.trim(),
  })
  saving.value = false
  if (!saved) {
    feedback.error(t.value['contacts.saveFailed'])
    return
  }
  dialogOpen.value = false
  feedback.success(t.value['contacts.saved'])
}

async function confirmDelete() {
  if (!deleteTarget.value || deleting.value) return
  deleting.value = true
  const ok = await contactsStore.removeContact(deleteTarget.value.id)
  deleting.value = false
  if (!ok) {
    feedback.error(t.value['contacts.deleteFailed'])
    return
  }
  deleteTarget.value = null
  feedback.success(t.value['contacts.deleted'])
}

onMounted(() => {
  void contactsStore.loadContacts()
})
</script>

<template>
  <component
    :is="props.embedded ? 'div' : UiScrollArea"
    class="contacts-view"
    :class="{ 'contacts-view--embedded': props.embedded }"
  >
    <div class="wrap">
      <section class="surface">
        <header class="surface__head contacts-head">
          <div class="title-row">
            <h2>{{ t['contacts.title'] }}</h2>
            <span class="local-chip">
              <i class="i-ri:hard-drive-2-line" aria-hidden="true" />
              {{ t['contacts.localOnly'] }}
            </span>
          </div>
          <div class="contact-head-actions">
            <UiSwitch
              class="contact-auto-collect"
              size="sm"
              :checked="preferences.autoCollectContacts"
              :title="t['contacts.autoCollectHint']"
              @update:checked="onAutoCollectChange"
            >
              {{ t['contacts.autoCollect'] }}
            </UiSwitch>
            <button type="button" class="btn btn--primary btn--sm" @click="openAdd">
              <i class="i-ri:user-add-line" aria-hidden="true" />
              {{ t['contacts.add'] }}
            </button>
          </div>
        </header>

        <div class="contact-toolbar">
          <div class="contact-search">
            <i class="i-ri:search-line" aria-hidden="true" />
            <input
              v-model="filter"
              type="search"
              spellcheck="false"
              :placeholder="t['contacts.search']"
              :aria-label="t['contacts.search']"
            />
            <button
              v-if="filter"
              type="button"
              class="search-clear"
              :aria-label="t['contacts.clearSearch']"
              @click="filter = ''"
            >
              <i class="i-ri:close-line" aria-hidden="true" />
            </button>
          </div>
        </div>

        <section class="contact-list">
          <div v-if="contactsStore.loading" class="state">
            <span class="spinner" />
          </div>
          <div v-else-if="visibleContacts.length === 0" class="state state--empty">
            <i class="i-ri:contacts-book-2-line" aria-hidden="true" />
            <p>{{ filter.trim() ? t['contacts.noMatch'] : t['contacts.empty'] }}</p>
          </div>
          <template v-else>
            <div v-for="contact in visibleContacts" :key="contact.id" class="contact-row">
              <span class="contact-avatar" aria-hidden="true">{{ initials(contact) }}</span>
              <div class="contact-info">
                <strong>{{ displayName(contact) }}</strong>
                <span>{{ contact.email }}</span>
              </div>
              <div class="contact-actions">
                <button
                  type="button"
                  class="icon-btn"
                  :title="t['contacts.edit']"
                  :aria-label="t['contacts.edit']"
                  @click="openEdit(contact)"
                >
                  <i class="i-ri:edit-2-line" aria-hidden="true" />
                </button>
                <button
                  type="button"
                  class="icon-btn contact-delete"
                  :title="t['contacts.delete']"
                  :aria-label="t['contacts.delete']"
                  @click="deleteTarget = contact"
                >
                  <i class="i-ri:delete-bin-5-line" aria-hidden="true" />
                </button>
              </div>
            </div>
          </template>
        </section>
      </section>
    </div>

    <UiDialog :open="dialogOpen" :busy="saving" size="narrow" @update:open="dialogOpen = $event">
      <UiDialogHeader
        :title="editing ? t['contacts.edit'] : t['contacts.new']"
        :close-label="t['cancel']"
        :disabled="saving"
        @close="dialogOpen = false"
      />
      <div class="dlg-body">
        <Field.Root class="field">
          <Field.Label class="field__label">{{ t['contacts.name'] }}</Field.Label>
          <Field.Input
            v-model="formName"
            class="input"
            type="text"
            :placeholder="t['contacts.namePlaceholder']"
          />
        </Field.Root>
        <Field.Root class="field">
          <Field.Label class="field__label">{{ t['contacts.email'] }}</Field.Label>
          <Field.Input
            v-model="formEmail"
            class="input"
            type="email"
            :placeholder="t['contacts.emailPlaceholder']"
            @keydown.enter="save"
          />
          <span v-if="formEmail.trim() && !emailValid" class="field-error">
            {{ t['contacts.invalidEmail'] }}
          </span>
        </Field.Root>
      </div>
      <footer class="dlg-foot">
        <button type="button" class="btn" :disabled="saving" @click="dialogOpen = false">
          {{ t['cancel'] }}
        </button>
        <button
          type="button"
          class="btn btn--primary"
          :disabled="!emailValid || saving"
          @click="save"
        >
          {{ t['contacts.save'] }}
        </button>
      </footer>
    </UiDialog>

    <ConfirmDialog
      :open="!!deleteTarget"
      :title="t['contacts.delete']"
      :description="
        deleteTarget ? tf('contacts.deleteConfirm', { name: displayName(deleteTarget) }) : undefined
      "
      :confirm-label="t['contacts.delete']"
      danger
      :busy="deleting"
      @update:open="(open) => (!open ? (deleteTarget = null) : undefined)"
      @confirm="confirmDelete"
    />
  </component>
</template>

<style scoped>
.contacts-view {
  height: 100%;
  color: var(--n-text-color);
}
.contacts-view--embedded {
  height: auto;
}
.contacts-head {
  gap: 12px;
}
.title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.contacts-head h2 {
  margin: 0;
}
.contact-head-actions {
  display: flex;
  flex: none;
  align-items: center;
  gap: 14px;
}
.local-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  font-size: 11px;
  color: var(--color-text-2);
  background: var(--n-action-color);
  border: 1px solid var(--n-border-color);
  border-radius: 999px;
}
.contact-toolbar {
  display: flex;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid var(--n-border-color);
}
.contact-search {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 8px;
  height: 30px;
  padding: 0 8px;
  color: var(--color-text-3);
  background: var(--n-action-color);
  border: 1px solid var(--n-border-color);
  border-radius: 6px;
}
.contact-auto-collect {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--color-text-2);
  font-size: 12px;
  cursor: pointer;
  user-select: none;
}
.contact-search:focus-within {
  color: var(--n-text-color);
  border-color: var(--n-primary-color);
}
.contact-search > i {
  flex: none;
  font-size: 15px;
}
.contact-search input {
  flex: 1;
  min-width: 0;
  height: 100%;
  padding: 0;
  color: inherit;
  background: transparent;
  border: none;
  outline: none;
  font: inherit;
  font-size: 13px;
}
.contact-search input::-webkit-search-cancel-button {
  display: none;
}
.search-clear {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  padding: 0;
  color: inherit;
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}
.search-clear:hover {
  color: var(--n-text-color);
  background: var(--n-sidebar-hover);
}
.contact-list {
  min-height: 0;
}
.contact-row {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 56px;
  padding: 9px 16px;
  border-bottom: 1px solid var(--n-border-color);
}
.contact-row:last-child {
  border-bottom: none;
}
.contact-row:hover {
  background: var(--n-sidebar-hover);
}
.contact-avatar {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  color: var(--n-primary-color);
  background: color-mix(in srgb, var(--n-primary-color) 16%, transparent);
  border: 1px solid color-mix(in srgb, var(--n-primary-color) 28%, var(--n-border-color));
  border-radius: 50%;
  font-size: 14px;
  font-weight: 700;
}
.contact-info {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.contact-info strong,
.contact-info span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.contact-info strong {
  font-size: 13px;
  font-weight: 600;
}
.contact-info span {
  font-size: 12px;
  color: var(--color-text-2);
}
.contact-actions {
  display: flex;
  flex: none;
  align-items: center;
  gap: 2px;
}
.contact-delete:hover {
  color: var(--color-danger);
  background: var(--color-danger-bg);
}
.state {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 34px 16px;
  color: var(--color-text-3);
  font-size: 13px;
}
.state--empty {
  flex-direction: column;
  gap: 8px;
}
.state--empty i {
  font-size: 26px;
  opacity: 0.65;
}
.state--empty p {
  margin: 0;
}
.field-error {
  font-size: 11px;
  color: var(--color-danger);
}
@media (max-width: 640px) {
  .contacts-head {
    align-items: stretch;
    flex-direction: column;
  }
  .contact-head-actions {
    justify-content: flex-end;
  }
}
</style>
