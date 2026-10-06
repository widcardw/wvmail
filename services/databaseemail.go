package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// Cached mail lists: folders, message summaries, folder metadata (for the paging
// totals), the read flag, and the searches that run against the cache.

// SaveFolders replaces all cached folders for an account.
func (m *DBManager) SaveFolders(accountID string, folders []FolderInfo) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}

	// Delete existing folders for this account
	if err := db.Where("account_id = ?", accountID).Delete(&CachedFolder{}).Error; err != nil {
		return err
	}

	// Insert new folders
	for _, f := range folders {
		attrsJSON, _ := json.Marshal(f.Attrs)
		cf := CachedFolder{
			AccountID: accountID,
			Name:      f.Name,
			Delim:     f.Delim,
			Attrs:     string(attrsJSON),
		}
		if err := db.Create(&cf).Error; err != nil {
			return fmt.Errorf("failed to save folder %q: %w", f.Name, err)
		}
	}
	return nil
}

// LoadFolders retrieves cached folders for an account.
func (m *DBManager) LoadFolders(accountID string) ([]FolderInfo, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}

	var cached []CachedFolder
	if err := db.Where("account_id = ?", accountID).
		Order("id ASC").
		Find(&cached).Error; err != nil {
		return nil, err
	}

	if len(cached) == 0 {
		return nil, nil
	}

	folders := make([]FolderInfo, 0, len(cached))
	for _, c := range cached {
		var attrs []string
		if c.Attrs != "" {
			json.Unmarshal([]byte(c.Attrs), &attrs)
		}
		folders = append(folders, FolderInfo{
			Name:  c.Name,
			Delim: c.Delim,
			Attrs: attrs,
		})
	}
	return folders, nil
}

// SaveEmails upserts cached email summaries for (account, folder) by UID without
// deleting older rows — so a refresh that only fetches the newest page won't wipe
// previously cached (older) messages. Removed emails may linger until purged.
func (m *DBManager) SaveEmails(accountID, folderName string, emails []EmailSummary) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}

	for _, e := range emails {
		// delete any previous row with the same (account, folder, uid), then insert fresh
		if err := db.Where("account_id = ? AND folder_name = ? AND uid = ?", accountID, folderName, e.UID).
			Delete(&CachedEmail{}).Error; err != nil {
			return err
		}
		flagsJSON, _ := json.Marshal(e.Flags)
		toJSON, _ := json.Marshal(e.To)
		ce := CachedEmail{
			AccountID:  accountID,
			FolderName: folderName,
			UID:        e.UID,
			Subject:    e.Subject,
			From:       e.From,
			FromName:   e.FromName,
			FromAddr:   e.FromAddr,
			To:         string(toJSON),
			Date:       e.Date,
			Flags:      string(flagsJSON),
			IsRead:     e.IsRead,
		}
		if err := db.Create(&ce).Error; err != nil {
			return fmt.Errorf("failed to save email uid=%d: %w", e.UID, err)
		}
	}
	return nil
}

// LoadEmails retrieves cached email summaries for a (account, folder).
func (m *DBManager) LoadEmails(accountID, folderName string) ([]EmailSummary, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}

	var cached []CachedEmail
	if err := db.Where("account_id = ? AND folder_name = ?", accountID, folderName).
		Order("date DESC").
		Find(&cached).Error; err != nil {
		return nil, err
	}

	if len(cached) == 0 {
		return nil, nil
	}

	emails := make([]EmailSummary, 0, len(cached))
	for _, c := range cached {
		emails = append(emails, cachedEmailSummary(c))
	}
	return emails, nil
}

// SearchEmails searches cached summaries and any cached message bodies. The
// cache is intentionally used instead of an external search index: it keeps
// the desktop app offline-friendly and is sufficient for the current data set.
func (m *DBManager) SearchEmails(query EmailSearchQuery) ([]EmailSummary, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}

	var cached []CachedEmail
	emailsQuery := db.Order("date DESC")
	if query.AccountID != "" {
		emailsQuery = emailsQuery.Where("account_id = ?", query.AccountID)
	}
	if query.FolderName != "" {
		emailsQuery = emailsQuery.Where("folder_name = ?", query.FolderName)
	}
	if err := emailsQuery.Find(&cached).Error; err != nil {
		return nil, err
	}

	var bodies []CachedEmailBody
	bodiesQuery := db
	if query.AccountID != "" {
		bodiesQuery = bodiesQuery.Where("account_id = ?", query.AccountID)
	}
	if query.FolderName != "" {
		bodiesQuery = bodiesQuery.Where("folder_name = ?", query.FolderName)
	}
	if err := bodiesQuery.Find(&bodies).Error; err != nil {
		return nil, err
	}
	bodyByKey := make(map[string]*CachedEmailBody, len(bodies))
	for i := range bodies {
		body := &bodies[i]
		bodyByKey[emailCacheKey(body.AccountID, body.FolderName, body.UID)] = body
	}

	results := make([]EmailSummary, 0, len(cached))
	for _, c := range cached {
		summary := cachedEmailSummary(c)
		summary.AccountID = c.AccountID
		summary.FolderName = c.FolderName
		body := bodyByKey[emailCacheKey(c.AccountID, c.FolderName, c.UID)]
		if searchEmailMatches(summary, body, query.Filters) {
			results = append(results, summary)
		}
	}
	return results, nil
}

// SuggestEmailContacts finds distinct cached senders or recipients matching
// a fragment. It only selects address-related columns and caps the result so
// this method remains inexpensive while the user types in the search box.
func (m *DBManager) SuggestEmailContacts(field, query, accountID string, limit int) ([]EmailContactSuggestion, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, err
	}

	field = strings.ToLower(strings.TrimSpace(field))
	needle := strings.ToLower(strings.TrimSpace(query))
	if (field != "from" && field != "to") || needle == "" {
		return []EmailContactSuggestion{}, nil
	}
	if limit < 1 {
		limit = 6
	}
	if limit > 20 {
		limit = 20
	}

	pattern := "%" + needle + "%"
	type contactCacheRow struct {
		From     string `gorm:"column:from"`
		FromName string `gorm:"column:from_name"`
		FromAddr string `gorm:"column:from_addr"`
		To       string `gorm:"column:to"`
		Date     string `gorm:"column:date"`
	}

	loadRows := func(model any) ([]contactCacheRow, error) {
		var rows []contactCacheRow
		query := db.Model(model).Select(`"from", from_name, from_addr, "to", date`)
		if accountID != "" {
			query = query.Where("account_id = ?", accountID)
		}
		if field == "from" {
			query = query.Where(`LOWER("from_addr") LIKE ? OR LOWER("from_name") LIKE ? OR LOWER("from") LIKE ?`, pattern, pattern, pattern)
		} else {
			query = query.Where(`LOWER("to") LIKE ?`, pattern)
		}
		if err := query.Order("date DESC").Find(&rows).Error; err != nil {
			return nil, err
		}
		return rows, nil
	}

	rows, err := loadRows(&CachedEmail{})
	if err != nil {
		return nil, err
	}
	// A recipient can also be present in a cached body when an older summary
	// did not contain the complete envelope.
	bodyRows, err := loadRows(&CachedEmailBody{})
	if err != nil {
		return nil, err
	}
	rows = append(rows, bodyRows...)

	results := make([]EmailContactSuggestion, 0, limit)
	seen := make(map[string]struct{}, limit)
	appendSuggestion := func(raw, name string) {
		address, parsedName := parseCachedContact(raw)
		if address == "" {
			return
		}
		if name == "" {
			name = parsedName
		}
		key := strings.ToLower(address)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		results = append(results, EmailContactSuggestion{Address: address, Name: strings.TrimSpace(name)})
	}

	for _, row := range rows {
		if len(results) >= limit {
			break
		}
		if field == "from" {
			appendSuggestion(row.FromAddr, row.FromName)
			if row.FromAddr == "" {
				appendSuggestion(row.From, row.FromName)
			}
			continue
		}

		var recipients []string
		if row.To != "" {
			_ = json.Unmarshal([]byte(row.To), &recipients)
		}
		for _, recipient := range recipients {
			appendSuggestion(recipient, "")
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

func parseCachedContact(raw string) (address, name string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	if start := strings.LastIndex(raw, "<"); start >= 0 {
		if end := strings.LastIndex(raw, ">"); end > start {
			address = strings.TrimSpace(raw[start+1 : end])
			name = strings.Trim(strings.TrimSpace(raw[:start]), `"'`)
		} else {
			address = raw
		}
	} else {
		address = strings.Trim(raw, `<> \t\r\n`)
	}
	if !strings.Contains(address, "@") {
		return "", ""
	}
	return address, name
}

func cachedEmailSummary(c CachedEmail) EmailSummary {
	var flags, to []string
	if c.Flags != "" {
		json.Unmarshal([]byte(c.Flags), &flags)
	}
	if c.To != "" {
		json.Unmarshal([]byte(c.To), &to)
	}
	return EmailSummary{
		UID:      c.UID,
		Subject:  c.Subject,
		From:     c.From,
		FromName: c.FromName,
		FromAddr: c.FromAddr,
		To:       to,
		Date:     c.Date,
		Flags:    flags,
		IsRead:   c.IsRead,
	}
}

func emailCacheKey(accountID, folderName string, uid uint32) string {
	return fmt.Sprintf("%s\x00%s\x00%d", accountID, folderName, uid)
}

// MarkEmailRead flips the cached read state of one message. The list styling and
// the unread count read this column, so opening a message should not have to wait
// for the next fetch before it stops counting as unread.
func (m *DBManager) MarkEmailRead(accountID, folderName string, uid uint32) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Model(&CachedEmail{}).
		Where("account_id = ? AND folder_name = ? AND uid = ?", accountID, folderName, uid).
		Update("is_read", true).Error
}

func searchEmailMatches(summary EmailSummary, body *CachedEmailBody, filters []EmailSearchFilter) bool {
	var fromValues, toValues, contentValues []string
	for _, filter := range filters {
		value := strings.TrimSpace(filter.Value)
		if value == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(filter.Field)) {
		case "from":
			fromValues = append(fromValues, value)
		case "to":
			toValues = append(toValues, value)
		case "content":
			contentValues = append(contentValues, value)
		}
	}

	fromText := []string{summary.From, summary.FromName, summary.FromAddr}
	if len(fromValues) > 0 && !containsAny(fromText, fromValues) {
		return false
	}

	recipients := append([]string{}, summary.To...)
	if body != nil && body.To != "" {
		var bodyRecipients []string
		json.Unmarshal([]byte(body.To), &bodyRecipients)
		recipients = append(recipients, bodyRecipients...)
	}
	if len(toValues) > 0 && !containsAny(recipients, toValues) {
		return false
	}

	contentText := []string{summary.Subject}
	if body != nil {
		contentText = append(contentText, body.BodyText, body.BodyHTML)
	}
	for _, value := range contentValues {
		matched := false
		for _, term := range strings.Split(value, "|") {
			term = strings.TrimSpace(term)
			if term != "" && containsAny(contentText, []string{term}) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func containsAny(values, queries []string) bool {
	for _, value := range values {
		lowerValue := strings.ToLower(value)
		for _, query := range queries {
			if strings.Contains(lowerValue, strings.ToLower(query)) {
				return true
			}
		}
	}
	return false
}

// SaveFolderMeta stores the total message count for a folder.
func (m *DBManager) SaveFolderMeta(accountID, folderName string, total int) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Where("account_id = ? AND folder_name = ?", accountID, folderName).
		Delete(&CachedFolderMeta{}).
		Session(&gorm.Session{SkipHooks: true}).
		Create(&CachedFolderMeta{AccountID: accountID, FolderName: folderName, Total: total}).Error
}

// LoadFolderMeta retrieves the cached total message count for a folder.
// Returns 0 if no meta is stored.
func (m *DBManager) LoadFolderMeta(accountID, folderName string) (int, error) {
	db, err := m.getDB()
	if err != nil {
		return 0, err
	}
	var meta CachedFolderMeta
	if err := db.Where("account_id = ? AND folder_name = ?", accountID, folderName).
		First(&meta).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return int(meta.Total), nil
}
