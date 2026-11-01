import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { ContactsService } from '#/wvmail/services'
import type { Contact } from '~/types/contact'

/**
 * The local address book. Contacts live only in this app's SQLite database and
 * are shared by the Contacts page, the message sender menu, and search ranking.
 */
export const useContactsStore = defineStore('contacts', () => {
  const contacts = ref<Contact[]>([])
  const loading = ref(false)
  const loaded = ref(false)

  const sortedContacts = computed(() =>
    [...contacts.value].sort((a, b) => {
      const byName = (a.name || a.email).localeCompare(b.name || b.email, undefined, {
        sensitivity: 'base',
      })
      return byName || a.email.localeCompare(b.email)
    }),
  )

  async function loadContacts(force = false) {
    if (loading.value || (loaded.value && !force)) return
    loading.value = true
    try {
      contacts.value = await ContactsService.LoadContacts()
      loaded.value = true
    } catch (error) {
      console.error('Failed to load contacts', error)
      contacts.value = []
    } finally {
      loading.value = false
    }
  }

  function hasEmail(email: string): boolean {
    const needle = email.trim().toLowerCase()
    return (
      Boolean(needle) && contacts.value.some((contact) => contact.email.toLowerCase() === needle)
    )
  }

  async function saveContact(contact: Omit<Contact, 'created_at' | 'updated_at'>) {
    try {
      const saved = await ContactsService.SaveContact({
        ...contact,
        created_at: 0,
        updated_at: 0,
      })
      const index = contacts.value.findIndex((item) => item.id === saved.id)
      if (index === -1) contacts.value = [...contacts.value, saved]
      else contacts.value[index] = saved
      loaded.value = true
      return saved
    } catch (error) {
      console.error('Failed to save contact', error)
      return null
    }
  }

  async function addContact(name: string, email: string) {
    return saveContact({ id: '', name: name.trim(), email: email.trim() })
  }

  async function removeContact(id: string) {
    const previous = contacts.value
    contacts.value = contacts.value.filter((contact) => contact.id !== id)
    try {
      await ContactsService.DeleteContact(id)
    } catch (error) {
      contacts.value = previous
      console.error('Failed to delete contact', error)
      return false
    }
    return true
  }

  return {
    contacts,
    loading,
    loaded,
    sortedContacts,
    loadContacts,
    hasEmail,
    saveContact,
    addContact,
    removeContact,
  }
})
