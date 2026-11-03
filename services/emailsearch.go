package services

import (
	"fmt"
)

// Searching the cached mail and suggesting contacts for the search fields.

// SearchEmails searches cached email summaries and cached bodies. Results are
// ordered newest first and paginated before being returned to the frontend.
func (s *EmailService) SearchEmails(query EmailSearchQuery, page, pageSize int) (*EmailPageResponse, error) {
	emails, err := s.db.SearchEmails(query)
	if err != nil {
		return nil, fmt.Errorf("search emails failed: %w", err)
	}
	page, pageSize = normalizePage(page, pageSize)
	return &EmailPageResponse{
		Emails: paginateSlices(emails, page, pageSize),
		Total:  len(emails),
	}, nil
}

// SuggestEmailContacts returns a small set of cached senders or recipients
// matching the typed fragment. Results are scoped to one account when
// accountID is provided; an empty accountID searches all cached accounts.
func (s *EmailService) SuggestEmailContacts(field, query, accountID string, limit int) ([]EmailContactSuggestion, error) {
	return s.db.SuggestEmailContacts(field, query, accountID, limit)
}
