package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

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
			MessageID:  e.MessageID,
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
		// A cached body carries its own copy of the date. Keep it in sync when
		// a list refresh replaces an implausible sender Date with InternalDate.
		if e.Date != "" {
			if err := db.Model(&CachedEmailBody{}).
				Where("account_id = ? AND folder_name = ? AND uid = ?", accountID, folderName, e.UID).
				Update("date", e.Date).Error; err != nil {
				return fmt.Errorf("failed to update cached body date for uid=%d: %w", e.UID, err)
			}
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
		Order("uid DESC").
		Find(&cached).Error; err != nil {
		return nil, err
	}

	if len(cached) == 0 {
		return nil, nil
	}

	emails := make([]EmailSummary, 0, len(cached))
	now := time.Now()
	for _, c := range cached {
		emails = append(emails, cachedEmailSummary(c, now))
	}
	// SQL can only order by arrival here: `date` is a string carrying each
	// sender's own offset, so ORDER BY date compares clock faces (see
	// emailtime.go). The instant comparison happens on the parsed values.
	sortEmailsNewestFirst(emails)
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
	emailsQuery := db.Order("uid DESC")
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
	now := time.Now()
	for _, c := range cached {
		summary := cachedEmailSummary(c, now)
		summary.AccountID = c.AccountID
		summary.FolderName = c.FolderName
		body := bodyByKey[emailCacheKey(c.AccountID, c.FolderName, c.UID)]
		if searchEmailMatches(summary, body, query.Filters) {
			results = append(results, summary)
		}
	}
	// The SQL pass only ordered by arrival; search results are read newest first
	// like the folder lists, which means comparing instants, not strings.
	sortEmailsNewestFirst(results)
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
		if err := query.Order("uid DESC").Find(&rows).Error; err != nil {
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
	// The caller keeps the first `limit` suggestions, so "recent senders first"
	// has to mean recent by the clock, not by the largest date string: the same
	// offset trap as the folder list (see emailtime.go). Ties keep the arrival
	// order the queries already applied.
	sort.SliceStable(rows, func(i, j int) bool {
		iTime, iOK := parseEmailTime(rows[i].Date)
		jTime, jOK := parseEmailTime(rows[j].Date)
		if iOK != jOK {
			return iOK
		}
		return iTime.After(jTime)
	})

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
		// An interpreted literal, not a raw one: in `<> \t\r\n` the escapes are
		// two characters each, which put `n`, `r`, `t` and `\` into the cutset and
		// turned "noreply@…" into "oreply@…".
		address = strings.Trim(raw, "<> \t\r\n")
	}
	if !strings.Contains(address, "@") {
		return "", ""
	}
	return address, name
}

func cachedEmailSummary(c CachedEmail, now time.Time) EmailSummary {
	var flags, to []string
	if c.Flags != "" {
		json.Unmarshal([]byte(c.Flags), &flags)
	}
	if c.To != "" {
		json.Unmarshal([]byte(c.To), &to)
	}
	return EmailSummary{
		UID:       c.UID,
		MessageID: c.MessageID,
		Subject:   c.Subject,
		From:      c.From,
		FromName:  c.FromName,
		FromAddr:  c.FromAddr,
		To:        to,
		Date:      sanitizeCachedEmailDate(c.Date, now),
		Flags:     flags,
		IsRead:    c.IsRead,
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

// SetEmailFlagged updates the cached flags after the server accepted the STORE.
// Both the list row and any cached body are updated: reopening a cached message
// must not restore the star state from before the toggle.
func (m *DBManager) SetEmailFlagged(accountID, folderName string, uid uint32, flagged bool) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}

	type cachedFlagRow struct {
		ID    uint
		Flags string
	}
	for _, model := range []any{&CachedEmail{}, &CachedEmailBody{}} {
		var rows []cachedFlagRow
		if err := db.Model(model).
			Select("id", "flags").
			Where("account_id = ? AND folder_name = ? AND uid = ?", accountID, folderName, uid).
			Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			var flags []string
			if row.Flags != "" {
				_ = json.Unmarshal([]byte(row.Flags), &flags)
			}
			encoded, err := json.Marshal(flagsWith(flags, "\\Flagged", flagged))
			if err != nil {
				return err
			}
			if err := db.Model(model).Where("id = ?", row.ID).Update("flags", string(encoded)).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteEmail removes one message from the summary and body caches and updates
// the folder's cached total. The server operation happens first in EmailService;
// this only mirrors an already accepted move/delete.
func (m *DBManager) DeleteEmail(accountID, folderName string, uid uint32) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("account_id = ? AND folder_name = ? AND uid = ?",
			accountID, folderName, uid).
			Delete(&CachedEmail{}).Error; err != nil {
			return err
		}
		if err := tx.Where("account_id = ? AND folder_name = ? AND uid = ?",
			accountID, folderName, uid).
			Delete(&CachedEmailBody{}).Error; err != nil {
			return err
		}
		return tx.Model(&CachedFolderMeta{}).
			Where("account_id = ? AND folder_name = ?", accountID, folderName).
			UpdateColumn("total", gorm.Expr("CASE WHEN total > 0 THEN total - 1 ELSE 0 END")).Error
	})
}

// PruneEmails removes cache rows that the server no longer reports in a folder.
// Unlike DeleteEmail, it must not touch the cached folder total: that total has
// already been refreshed from the server.
func (m *DBManager) PruneEmails(accountID, folderName string, uids []uint32) error {
	if len(uids) == 0 {
		return nil
	}
	db, err := m.getDB()
	if err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("account_id = ? AND folder_name = ? AND uid IN ?",
			accountID, folderName, uids).
			Delete(&CachedEmail{}).Error; err != nil {
			return err
		}
		return tx.Where("account_id = ? AND folder_name = ? AND uid IN ?",
			accountID, folderName, uids).
			Delete(&CachedEmailBody{}).Error
	})
}

// flagsWith adds or removes one IMAP flag without changing the order of the
// others. IMAP flag names are case-insensitive.
func flagsWith(flags []string, flag string, enabled bool) []string {
	out := make([]string, 0, len(flags)+1)
	found := false
	for _, current := range flags {
		if strings.EqualFold(current, flag) {
			found = true
			if !enabled {
				continue
			}
		}
		out = append(out, current)
	}
	if enabled && !found {
		out = append(out, flag)
	}
	return out
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

// FolderMeta is what one fetch of a folder left in the cache: how many messages
// it held and the mailbox's next UID at that moment.
//
// A zero UIDNext means "unknown" — a row written before UIDNEXT was cached, or a
// server that did not report it — and callers must treat it as "no reference
// point" rather than as a real value.
type FolderMeta struct {
	Total   int
	UIDNext uint32
}

// SaveFolderMeta stores the message count and UIDNEXT a fetch just observed.
func (m *DBManager) SaveFolderMeta(accountID, folderName string, meta FolderMeta) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Where("account_id = ? AND folder_name = ?", accountID, folderName).
		Delete(&CachedFolderMeta{}).
		Session(&gorm.Session{SkipHooks: true}).
		Create(&CachedFolderMeta{
			AccountID:  accountID,
			FolderName: folderName,
			Total:      meta.Total,
			UIDNext:    meta.UIDNext,
		}).Error
}

// LoadFolderMeta retrieves the cached state of a folder.
// Returns the zero FolderMeta if none is stored.
func (m *DBManager) LoadFolderMeta(accountID, folderName string) (FolderMeta, error) {
	db, err := m.getDB()
	if err != nil {
		return FolderMeta{}, err
	}
	var meta CachedFolderMeta
	if err := db.Where("account_id = ? AND folder_name = ?", accountID, folderName).
		First(&meta).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return FolderMeta{}, nil
		}
		return FolderMeta{}, err
	}
	return FolderMeta{Total: int(meta.Total), UIDNext: meta.UIDNext}, nil
}

// SaveFlaggedSnapshot replaces the account's cached All Starred result.
func (m *DBManager) SaveFlaggedSnapshot(accountID string, emails []EmailSummary, fetchedAt int64) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(emails)
	if err != nil {
		return err
	}
	snapshot := CachedFlaggedSnapshot{
		AccountID: accountID,
		Emails:    string(encoded),
		FetchedAt: fetchedAt,
	}
	return db.Save(&snapshot).Error
}

// LoadFlaggedSnapshot reads the account's cached All Starred result. A missing
// row means the account has not completed its first search yet.
func (m *DBManager) LoadFlaggedSnapshot(accountID string) ([]EmailSummary, int64, error) {
	db, err := m.getDB()
	if err != nil {
		return nil, 0, err
	}
	var snapshot CachedFlaggedSnapshot
	if err := db.Where("account_id = ?", accountID).First(&snapshot).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, nil
		}
		return nil, 0, err
	}
	var emails []EmailSummary
	if snapshot.Emails != "" {
		if err := json.Unmarshal([]byte(snapshot.Emails), &emails); err != nil {
			return nil, 0, err
		}
	}
	return emails, snapshot.FetchedAt, nil
}

// InvalidateFlaggedSnapshot marks an account's aggregate stale after a star
// change. The next open paints the old rows immediately, then refreshes them.
func (m *DBManager) InvalidateFlaggedSnapshot(accountID string) error {
	db, err := m.getDB()
	if err != nil {
		return err
	}
	return db.Model(&CachedFlaggedSnapshot{}).
		Where("account_id = ?", accountID).
		Update("fetched_at", 0).Error
}

// DeleteAccountCache removes every cached row belonging to one account: folders,
// folder metadata, message summaries and their bodies.
//
// Deleting an account does not have to do this — the rows become unreachable the
// moment the account leaves config.yaml, because everything here is keyed by
// account_id and a re-added mailbox gets a fresh UUID. They do take real disk
// though (bodies and attachment metadata are text), which is why the settings page
// offers it as a checkbox rather than doing it silently.
//
// One transaction: a failure halfway would leave a mailbox with bodies but no
// summaries, which reads as corruption in the search path.
func (m *DBManager) DeleteAccountCache(accountID string) (int64, error) {
	db, err := m.getDB()
	if err != nil {
		return 0, err
	}
	var deleted int64
	err = db.Transaction(func(tx *gorm.DB) error {
		for _, model := range []any{
			&CachedEmail{},
			&CachedEmailBody{},
			&CachedFolder{},
			&CachedFolderMeta{},
			&CachedFlaggedSnapshot{},
		} {
			res := tx.Where("account_id = ?", accountID).Delete(model)
			if res.Error != nil {
				return res.Error
			}
			deleted += res.RowsAffected
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}
