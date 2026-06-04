package services

import "fmt"

// Note represents a single rich-text note exposed to the frontend.
type Note struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// NotesService provides CRUD operations for notes stored in the local SQLite database.
type NotesService struct {
	db *DBManager
}

func NewNotesService() *NotesService {
	return &NotesService{
		db: NewDBManager(),
	}
}

// SaveNote creates or updates a note. If the note has an empty ID, one is assigned.
func (s *NotesService) SaveNote(note Note) (Note, error) {
	if note.ID == "" {
		return note, fmt.Errorf("note ID is required")
	}
	cn := CachedNote{
		ID:      note.ID,
		Title:   note.Title,
		Content: note.Content,
	}
	if err := s.db.SaveNote(&cn); err != nil {
		return note, fmt.Errorf("failed to save note: %w", err)
	}
	return s.toNote(cn), nil
}

// LoadNotes returns all notes, ordered by updated_at DESC.
func (s *NotesService) LoadNotes() ([]Note, error) {
	cached, err := s.db.LoadNotes()
	if err != nil {
		return nil, fmt.Errorf("failed to load notes: %w", err)
	}
	notes := make([]Note, len(cached))
	for i, cn := range cached {
		notes[i] = s.toNote(cn)
	}
	return notes, nil
}

// DeleteNote removes a note by ID.
func (s *NotesService) DeleteNote(id string) error {
	if err := s.db.DeleteNote(id); err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}
	return nil
}

func (s *NotesService) toNote(cn CachedNote) Note {
	return Note{
		ID:        cn.ID,
		Title:     cn.Title,
		Content:   cn.Content,
		CreatedAt: cn.CreatedAt,
		UpdatedAt: cn.UpdatedAt,
	}
}
