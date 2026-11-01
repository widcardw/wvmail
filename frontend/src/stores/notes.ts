import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { NotesService } from '#/wvmail/services'

export interface Note {
  id: string
  title: string
  content: string
  created_at: number
  updated_at: number
}

export const useNotesStore = defineStore('notes', () => {
  const notes = ref<Note[]>([])
  const loading = ref(false)
  const selectedId = ref<string | null>(null)

  const selectedNote = computed(() => {
    if (!selectedId.value) return null
    return notes.value.find((n) => n.id === selectedId.value) ?? null
  })

  const sortedNotes = computed(() =>
    // Stable tiebreak: several saves can land in the same second, and without it the
    // sidebar order would shuffle between reloads.
    [...notes.value].sort((a, b) =>
      b.updated_at === a.updated_at ? a.id.localeCompare(b.id) : b.updated_at - a.updated_at,
    ),
  )

  async function loadNotes() {
    loading.value = true
    try {
      notes.value = await NotesService.LoadNotes()
    } catch (e) {
      console.error('Failed to load notes', e)
      notes.value = []
    } finally {
      loading.value = false
    }
  }

  function selectNote(id: string) {
    selectedId.value = id
  }

  async function createNote(): Promise<string | null> {
    const id = crypto.randomUUID()
    try {
      await NotesService.SaveNote({
        id,
        title: '',
        content: '',
        created_at: 0,
        updated_at: Math.floor(Date.now() / 1000),
      })
      await loadNotes()
      selectedId.value = id
      return id
    } catch (e) {
      console.error('Failed to create note', e)
      return null
    }
  }

  /**
   * Save one note by id. The id is explicit — never "the selected note" — because the
   * editor flushes a pending edit while switching away, when `selectedId` already
   * points at the next note.
   */
  async function saveNote(id: string, title: string, content: string) {
    const now = Math.floor(Date.now() / 1000)
    try {
      await NotesService.SaveNote({ id, title, content, created_at: 0, updated_at: now })
      const idx = notes.value.findIndex((n) => n.id === id)
      if (idx !== -1) {
        notes.value[idx] = { ...notes.value[idx], title, content, updated_at: now }
      }
    } catch (e) {
      console.error('Failed to save note', e)
    }
  }

  async function deleteNote(id: string): Promise<boolean> {
    try {
      await NotesService.DeleteNote(id)
      notes.value = notes.value.filter((note) => note.id !== id)
      if (selectedId.value === id) selectedId.value = null
      return true
    } catch (e) {
      console.error('Failed to delete note', e)
      return false
    }
  }

  return {
    notes,
    loading,
    selectedId,
    selectedNote,
    sortedNotes,
    loadNotes,
    selectNote,
    createNote,
    saveNote,
    deleteNote,
  }
})
