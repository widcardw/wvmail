package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/mail"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
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
	db  *DBManager
	cfg *ConfigService
	mu  sync.Mutex
}

func NewContactsService(cfg ...*ConfigService) *ContactsService {
	var config *ConfigService
	if len(cfg) > 0 {
		config = cfg[0]
	}
	return &ContactsService{db: NewDBManager(), cfg: config}
}

// SaveContact creates or updates a contact. An empty ID means "new"; saving an
// address that already exists returns the existing row instead of duplicating it.
func (s *ContactsService) SaveContact(contact Contact) (Contact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveContact(contact, true)
}

func (s *ContactsService) saveContact(contact Contact, clearExclusion bool) (Contact, error) {
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
			if clearExclusion {
				_ = s.db.RemoveContactExclusion(email)
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
	if clearExclusion {
		if current != nil && !strings.EqualFold(current.Email, email) {
			_ = s.db.AddContactExclusion(current.Email)
		}
		_ = s.db.RemoveContactExclusion(email)
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
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("contact ID is required")
	}
	current, err := s.db.LoadContactByID(id)
	if err != nil {
		return fmt.Errorf("failed to load contact: %w", err)
	}
	if current == nil {
		return nil
	}
	if err := s.db.DeleteContactAndExclude(id, current.Email); err != nil {
		return fmt.Errorf("failed to delete contact: %w", err)
	}
	return nil
}

var contactRoleLocalParts = map[string]bool{
	"noreply":       true,
	"no-reply":      true,
	"no_reply":      true,
	"donotreply":    true,
	"do-not-reply":  true,
	"do_not_reply":  true,
	"mailer-daemon": true,
	"postmaster":    true,
	"notifications": true,
	"notification":  true,
	"notify":        true,
	"alerts":        true,
	"alert":         true,
	"bounces":       true,
	"bounce":        true,
	"auto-reply":    true,
	"autoreply":     true,
	"automated":     true,
	"newsletter":    true,
	"marketing":     true,
	"promotions":    true,
	"promo":         true,
	"updates":       true,
	"update":        true,
	"system":        true,
	"security":      true,
	"digest":        true,
}

var contactRolePrefixes = []string{
	"noreply",
	"no-reply",
	"no_reply",
	"donotreply",
	"do-not-reply",
	"do_not_reply",
	"notify",
	"notification",
	"alert",
	"mailer-daemon",
	"bounce",
	"auto-reply",
	"autoreply",
	"newsletter",
	"marketing",
	"promo",
}

// CollectFromMail adds senders from newly fetched INBOX messages to the local
// address book. The first observation of a folder only records a UID baseline;
// later fetches collect addresses above it. It is wired to EmailService and is
// intentionally not a frontend-facing Wails method.
//
//wails:ignore
func (s *ContactsService) CollectFromMail(
	account EmailAccount,
	folder string,
	messages []EmailSummary,
) (int, error) {
	if s.cfg == nil || !isInbox(folder) {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.db.LoadContactCollectionState(account.ID, folder)
	if err != nil {
		return 0, fmt.Errorf("load contact collection state: %w", err)
	}
	maxUID := maxEmailUID(messages)
	if state == nil {
		if err := s.db.SaveContactCollectionState(&CachedContactCollectionState{
			AccountID:  account.ID,
			FolderName: folder,
			LastUID:    maxUID,
		}); err != nil {
			return 0, fmt.Errorf("save contact collection baseline: %w", err)
		}
		return 0, nil
	}
	if maxUID <= state.LastUID {
		return 0, nil
	}

	cfg, err := s.cfg.GetConfig()
	if err != nil {
		return 0, fmt.Errorf("load contact collection config: %w", err)
	}
	if !cfg.AutoCollectContacts {
		state.LastUID = maxUID
		return 0, s.db.SaveContactCollectionState(state)
	}

	ownEmails := contactOwnEmails(account, cfg.EmailAccounts)
	excluded, err := s.db.LoadContactExclusionEmails()
	if err != nil {
		return 0, fmt.Errorf("load contact exclusions: %w", err)
	}

	added := 0
	defer func() {
		if added > 0 {
			notifyContactsUpdated(added)
		}
	}()
	for _, message := range messages {
		if message.UID <= state.LastUID || message.IsBulk {
			continue
		}
		email, name, ok := autoContactIdentity(message)
		if !ok || ownEmails[email] || excluded[email] || !collectableContactEmail(email) {
			continue
		}

		existing, err := s.db.LoadContactByEmail(email)
		if err != nil {
			return added, fmt.Errorf("look up auto-collected contact: %w", err)
		}
		if existing != nil {
			if existing.Name == "" && name != "" {
				existing.Name = name
				if err := s.db.SaveContact(existing); err != nil {
					return added, fmt.Errorf("fill auto-collected contact name: %w", err)
				}
			}
			continue
		}
		if _, err := s.saveContact(Contact{Name: name, Email: email}, false); err != nil {
			return added, fmt.Errorf("save auto-collected contact: %w", err)
		}
		added++
	}

	state.LastUID = maxUID
	if err := s.db.SaveContactCollectionState(state); err != nil {
		return added, fmt.Errorf("save contact collection state: %w", err)
	}
	return added, nil
}

func maxEmailUID(messages []EmailSummary) uint32 {
	var maxUID uint32
	for _, message := range messages {
		if message.UID > maxUID {
			maxUID = message.UID
		}
	}
	return maxUID
}

func contactOwnEmails(account EmailAccount, accounts []EmailAccount) map[string]bool {
	out := make(map[string]bool, len(accounts)*2+2)
	add := func(raw string) {
		if email, _, err := normalizeContactEmail(raw); err == nil {
			out[email] = true
		}
	}
	add(account.Email)
	add(account.IMAPUser)
	for _, configured := range accounts {
		add(configured.Email)
		add(configured.IMAPUser)
	}
	return out
}

func autoContactIdentity(message EmailSummary) (email, name string, ok bool) {
	raw := strings.TrimSpace(message.FromAddr)
	if raw == "" {
		raw = strings.TrimSpace(message.From)
	}
	email, parsedName, err := normalizeContactEmail(raw)
	if err != nil {
		return "", "", false
	}
	name = strings.TrimSpace(message.FromName)
	if name == "" {
		name = parsedName
	}
	if name == "" {
		name = strings.SplitN(email, "@", 2)[0]
	}
	return email, name, true
}

func collectableContactEmail(email string) bool {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return false
	}
	local := strings.ToLower(email[:at])
	if contactRoleLocalParts[local] {
		return false
	}
	for _, prefix := range contactRolePrefixes {
		if strings.HasPrefix(local, prefix) {
			return false
		}
	}
	return true
}

func notifyContactsUpdated(added int) {
	app := application.Get()
	if app != nil {
		app.Event.Emit("contacts-updated", added)
	}
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
