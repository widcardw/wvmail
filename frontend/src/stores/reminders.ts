import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { RemindersService } from '#/wvmail/services'
import type { Reminder } from '~/types/reminder'

/**
 * Reminders live in SQLite (see services/reminderservice.go). Both the reminder
 * page and the "this date in the mail → add reminder" flow write through here, so
 * they share one ordered list instead of each keeping its own copy.
 */
export const useRemindersStore = defineStore('reminders', () => {
  const reminders = ref<Reminder[]>([])
  const loading = ref(false)
  const loaded = ref(false)

  /** Soonest first — the storage order, made explicit for the views. */
  const sorted = computed(() => [...reminders.value].sort((a, b) => a.time - b.time))

  async function loadReminders(force = false) {
    if (loading.value || (loaded.value && !force)) return
    loading.value = true
    try {
      reminders.value = await RemindersService.LoadReminders()
      loaded.value = true
    } catch (e) {
      console.error('Failed to load reminders', e)
    } finally {
      loading.value = false
    }
  }

  /** Creates a reminder. Returns the stored row, or null when it could not be saved. */
  async function add(title: string, time: number): Promise<Reminder | null> {
    const trimmed = title.trim()
    if (!trimmed) return null
    try {
      // An empty id asks the service to assign one.
      const saved = await RemindersService.SaveReminder({
        id: '',
        title: trimmed,
        time,
        done: false,
        created_at: 0,
        updated_at: 0,
      })
      const reminder: Reminder = saved
      reminders.value = [...reminders.value, reminder]
      return reminder
    } catch (e) {
      console.error('Failed to save reminder', e)
      return null
    }
  }

  /** Flips (or sets) the done flag; the local list updates first, then rolls back on failure. */
  async function toggle(id: string, done?: boolean) {
    const reminder = reminders.value.find((r) => r.id === id)
    if (!reminder) return
    const previous = reminder.done
    const next = done ?? !previous
    reminder.done = next
    try {
      await RemindersService.SetReminderDone(id, next)
    } catch (e) {
      reminder.done = previous
      console.error('Failed to update reminder', e)
    }
  }

  async function remove(id: string) {
    const previous = reminders.value
    reminders.value = reminders.value.filter((r) => r.id !== id)
    try {
      await RemindersService.DeleteReminder(id)
    } catch (e) {
      reminders.value = previous
      console.error('Failed to delete reminder', e)
    }
  }

  return { reminders, loading, loaded, sorted, loadReminders, add, toggle, remove }
})
