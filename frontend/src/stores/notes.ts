import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { NotesService } from '../../bindings/wvmail/services'

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
  const currentContent = ref('')

  const selectedNote = computed(() => {
    if (!selectedId.value) return null
    return notes.value.find((n) => n.id === selectedId.value) ?? null
  })

  const sortedNotes = computed(() => {
    return [...notes.value].sort((a, b) => b.updated_at - a.updated_at)
  })

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

  async function selectNote(id: string) {
    selectedId.value = id
    const note = notes.value.find((n) => n.id === id)
    currentContent.value = note?.content ?? ''
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
      currentContent.value = ''
      return id
    } catch (e) {
      console.error('Failed to create note', e)
      return null
    }
  }

  async function saveCurrentNote(title: string, content: string) {
    if (!selectedId.value) return
    const now = Math.floor(Date.now() / 1000)
    try {
      await NotesService.SaveNote({
        id: selectedId.value,
        title,
        content,
        created_at: 0,
        updated_at: now,
      })
      // Update local state
      const idx = notes.value.findIndex((n) => n.id === selectedId.value)
      if (idx !== -1) {
        notes.value[idx] = { ...notes.value[idx], title, content, updated_at: now }
      }
    } catch (e) {
      console.error('Failed to save note', e)
    }
  }

  async function deleteNote(id: string) {
    try {
      await NotesService.DeleteNote(id)
      if (selectedId.value === id) {
        selectedId.value = null
        currentContent.value = ''
      }
      await loadNotes()
    } catch (e) {
      console.error('Failed to delete note', e)
    }
  }

  return {
    notes,
    loading,
    selectedId,
    currentContent,
    selectedNote,
    sortedNotes,
    loadNotes,
    selectNote,
    createNote,
    saveCurrentNote,
    deleteNote,
  }
})
