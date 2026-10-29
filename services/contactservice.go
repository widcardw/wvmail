package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/mail"
	"strings"
)

// Contact is one locally saved address-book entry.
type Contact struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// ContactsService provides CRUD operations for contacts stored only in the
// local SQLite database. Nothing here talks to an address-book server.
type ContactsService struct {
	db *DBManager
}

func NewContactsService() *ContactsService {
	return &ContactsService{db: NewDBManager()}
}

// SaveContact creates or updates a contact. An empty ID means "new"; saving an
// address that already exists returns the existing row instead of duplicating it.
func (s *ContactsService) SaveContact(contact Contact) (Contact, error) {
	email, parsedName, err := normalizeContactEmail(contact.Email)
	if err != nil {
		return contact, err
	}
	name := strings.TrimSpace(contact.Name)
	if name == "" {
		name = parsedName
	}

	existing, err := s.db.LoadContactByEmail(email)
	if err != nil {
		return contact, fmt.Errorf("failed to look up contact: %w", err)
	}
	if contact.ID == "" {
		if existing != nil {
			if existing.Name == "" && name != "" {
				existing.Name = name
				if err := s.db.SaveContact(existing); err != nil {
					return contact, fmt.Errorf("failed to save contact: %w", err)
				}
			}
			return s.toContact(*existing), nil
		}
		contact.ID, err = newContactID()
		if err != nil {
			return contact, fmt.Errorf("failed to generate contact ID: %w", err)
		}
	} else if existing != nil && existing.ID != contact.ID {
		return contact, fmt.Errorf("another contact already uses %s", email)
	}

	var current *CachedContact
	if contact.ID != "" {
		current, err = s.db.LoadContactByID(contact.ID)
		if err != nil {
			return contact, fmt.Errorf("failed to look up contact: %w", err)
		}
	}

	saved := CachedContact{ID: contact.ID, Name: name, Email: email}
	// Save updates every column. Keep the original creation time instead of
	// writing the zero value from the request back over it.
	if current != nil {
		saved.CreatedAt = current.CreatedAt
	}
	if err := s.db.SaveContact(&saved); err != nil {
		return contact, fmt.Errorf("failed to save contact: %w", err)
	}
	return s.toContact(saved), nil
}

// LoadContacts returns every local contact in display order.
func (s *ContactsService) LoadContacts() ([]Contact, error) {
	cached, err := s.db.LoadContacts()
	if err != nil {
		return nil, fmt.Errorf("failed to load contacts: %w", err)
	}
	contacts := make([]Contact, len(cached))
	for i, contact := range cached {
		contacts[i] = s.toContact(contact)
	}
	return contacts, nil
}

// DeleteContact removes a contact by ID.
func (s *ContactsService) DeleteContact(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("contact ID is required")
	}
	if err := s.db.DeleteContact(id); err != nil {
		return fmt.Errorf("failed to delete contact: %w", err)
	}
	return nil
}

func (s *ContactsService) toContact(contact CachedContact) Contact {
	return Contact{
		ID:        contact.ID,
		Name:      contact.Name,
		Email:     contact.Email,
		CreatedAt: contact.CreatedAt,
		UpdatedAt: contact.UpdatedAt,
	}
}

func normalizeContactEmail(raw string) (email, name string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("contact email is required")
	}
	parsed, err := mail.ParseAddress(raw)
	if err != nil || !strings.Contains(parsed.Address, "@") {
		return "", "", fmt.Errorf("invalid contact email")
	}
	return strings.ToLower(parsed.Address), strings.TrimSpace(parsed.Name), nil
}

func newContactID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
