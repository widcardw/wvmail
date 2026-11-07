package services

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Conversation grouping is local because IMAP THREAD is an optional extension
// and providers implement different algorithms. References/In-Reply-To are the
// authoritative links; subject + participants are only a fallback for old mail
// that never carried those headers.
const weakThreadWindow = 14 * 24 * time.Hour

var (
	threadSubjectPrefix = regexp.MustCompile(
		`(?i)^\s*(?:(?:re|fw|fwd|aw|sv|回复|转发)\s*(?:\[\d+\])?\s*[:：]\s*)+`,
	)
	threadReplyPrefix = regexp.MustCompile(
		`(?i)^\s*(?:re|fw|fwd|aw|sv|回复|转发)\s*(?:\[\d+\])?\s*[:：]`,
	)
)

// applyEmailThreads decorates every message with the stable ID and counters of
// its conversation. It mutates the slice in place and is intentionally run on
// the full cached folder, not on one page, so a reply cannot lose its parent
// merely because the two rows arrived on different pages.
func applyEmailThreads(emails []EmailSummary) {
	if len(emails) == 0 {
		return
	}

	parent := make([]int, len(emails))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}

	byMessageID := make(map[string]int, len(emails))
	for i := range emails {
		id := normalizeMessageID(emails[i].MessageID)
		if id == "" {
			continue
		}
		if existing, ok := byMessageID[id]; ok {
			union(existing, i)
		} else {
			byMessageID[id] = i
		}
	}

	for i := range emails {
		for _, ref := range appendMessageIDs(emails[i].References, emails[i].InReplyTo) {
			if parentIndex, ok := byMessageID[normalizeMessageID(ref)]; ok {
				union(i, parentIndex)
			}
		}
	}

	// Two replies can share a parent that is not in the cache (for example the
	// original is on an older page). The shared reference still proves they are
	// one conversation, so remember the first message that mentioned each ID.
	byReference := make(map[string]int)
	for i := range emails {
		for _, ref := range appendMessageIDs(emails[i].References, emails[i].InReplyTo) {
			id := normalizeMessageID(ref)
			if id == "" {
				continue
			}
			if existing, ok := byReference[id]; ok {
				union(existing, i)
			} else {
				byReference[id] = i
			}
		}
	}

	// Weak matching is deliberately disabled as soon as either side has a
	// usable reply header: otherwise an unrelated newsletter with the same
	// subject could be pulled into a real conversation. Bucket by normalized
	// subject first so a large cache does not pay a quadratic scan across every
	// unrelated message.
	weakBySubject := make(map[string][]int)
	for i := range emails {
		if emails[i].IsBulk || hasThreadHeaders(emails[i]) {
			continue
		}
		subject := normalizeThreadSubject(emails[i].Subject)
		if subject == "" {
			continue
		}
		weakBySubject[subject] = append(weakBySubject[subject], i)
	}
	for _, candidates := range weakBySubject {
		for left := 0; left < len(candidates); left++ {
			for right := left + 1; right < len(candidates); right++ {
				i, j := candidates[left], candidates[right]
				if find(i) == find(j) || !weakThreadMatch(emails[i], emails[j]) {
					continue
				}
				union(i, j)
			}
		}
	}

	groups := make(map[int][]int)
	for i := range emails {
		root := find(i)
		groups[root] = append(groups[root], i)
	}

	for _, indexes := range groups {
		sort.Slice(indexes, func(i, j int) bool {
			ti, tj := parsedThreadDate(emails[indexes[i]].Date), parsedThreadDate(emails[indexes[j]].Date)
			if ti.Equal(tj) {
				return emails[indexes[i]].UID < emails[indexes[j]].UID
			}
			return ti.Before(tj)
		})

		threadID := stableThreadID(emails, indexes)
		unread := 0
		for _, index := range indexes {
			if !emails[index].IsRead {
				unread++
			}
		}
		for _, index := range indexes {
			emails[index].ThreadID = threadID
			emails[index].ThreadCount = len(indexes)
			emails[index].ThreadUnread = unread
		}
	}
}

func weakThreadMatch(a, b EmailSummary) bool {
	if a.IsBulk || b.IsBulk {
		return false
	}
	if hasThreadHeaders(a) || hasThreadHeaders(b) {
		return false
	}
	subject := normalizeThreadSubject(a.Subject)
	if subject == "" || subject != normalizeThreadSubject(b.Subject) {
		return false
	}
	if !threadParticipantsOverlap(a, b) {
		return false
	}
	if !sameThreadParticipants(a, b) &&
		!threadReplyPrefix.MatchString(a.Subject) &&
		!threadReplyPrefix.MatchString(b.Subject) {
		return false
	}
	return withinWeakThreadWindow(a.Date, b.Date)
}

func hasThreadHeaders(email EmailSummary) bool {
	return len(email.InReplyTo) > 0 || len(email.References) > 0
}

func withinWeakThreadWindow(a, b string) bool {
	ta, tb := parsedThreadDate(a), parsedThreadDate(b)
	if ta.IsZero() || tb.IsZero() {
		return false
	}
	diff := ta.Sub(tb)
	if diff < 0 {
		diff = -diff
	}
	return diff <= weakThreadWindow
}

func parsedThreadDate(value string) time.Time {
	parsed, ok := parseEmailTime(value)
	if !ok {
		return time.Time{}
	}
	return parsed
}

func normalizeThreadSubject(subject string) string {
	subject = threadSubjectPrefix.ReplaceAllString(strings.TrimSpace(subject), "")
	return strings.Join(strings.Fields(strings.ToLower(subject)), " ")
}

func normalizeMessageID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "<")
	id = strings.TrimSuffix(id, ">")
	return strings.ToLower(strings.TrimSpace(id))
}

func appendMessageIDs(lists ...[]string) []string {
	total := 0
	for _, list := range lists {
		total += len(list)
	}
	out := make([]string, 0, total)
	for _, list := range lists {
		out = append(out, list...)
	}
	return out
}

func threadParticipants(email EmailSummary) map[string]struct{} {
	participants := make(map[string]struct{})
	add := func(raw string) {
		if address := normalizeThreadAddress(raw); address != "" {
			participants[address] = struct{}{}
		}
	}
	add(email.FromAddr)
	if email.FromAddr == "" {
		add(email.From)
	}
	for _, address := range email.To {
		add(address)
	}
	return participants
}

func normalizeThreadAddress(raw string) string {
	raw = strings.TrimSpace(raw)
	if start, end := strings.LastIndex(raw, "<"), strings.LastIndex(raw, ">"); start >= 0 && end > start {
		raw = raw[start+1 : end]
	}
	return strings.ToLower(strings.TrimSpace(raw))
}

func threadParticipantsOverlap(a, b EmailSummary) bool {
	left, right := threadParticipants(a), threadParticipants(b)
	for address := range left {
		if _, ok := right[address]; ok {
			return true
		}
	}
	return false
}

func sameThreadParticipants(a, b EmailSummary) bool {
	left, right := threadParticipants(a), threadParticipants(b)
	if len(left) == 0 || len(left) != len(right) {
		return false
	}
	for address := range left {
		if _, ok := right[address]; !ok {
			return false
		}
	}
	return true
}

func stableThreadID(emails []EmailSummary, indexes []int) string {
	email := emails[indexes[0]]
	if len(email.References) > 0 {
		if id := normalizeMessageID(email.References[0]); id != "" {
			return "mid:" + id
		}
	}
	if len(email.InReplyTo) > 0 {
		if id := normalizeMessageID(email.InReplyTo[0]); id != "" {
			return "mid:" + id
		}
	}
	if id := normalizeMessageID(email.MessageID); id != "" {
		return "mid:" + id
	}
	subject := normalizeThreadSubject(email.Subject)
	if subject != "" {
		participants := make(map[string]struct{})
		for _, index := range indexes {
			for address := range threadParticipants(emails[index]) {
				participants[address] = struct{}{}
			}
		}
		addresses := make([]string, 0, len(participants))
		for address := range participants {
			addresses = append(addresses, address)
		}
		sort.Strings(addresses)
		sum := sha1.Sum([]byte(subject + "\x00" + strings.Join(addresses, "\x00")))
		return "subj:" + hex.EncodeToString(sum[:8])
	}
	return fmt.Sprintf("uid:%s:%s:%d", email.AccountID, email.FolderName, email.UID)
}

// ListThreadEmails returns every cached message in one conversation, oldest
// first so the reading pane's tabs read like a conversation. The folder cache
// is the source: opening a thread never costs another IMAP round trip.
func (s *EmailService) ListThreadEmails(
	account EmailAccount,
	folder string,
	threadID string,
) ([]EmailSummary, error) {
	if strings.TrimSpace(threadID) == "" {
		return []EmailSummary{}, nil
	}
	emails, err := s.db.LoadEmails(account.ID, folder)
	if err != nil {
		return nil, err
	}
	thread := make([]EmailSummary, 0)
	for _, email := range emails {
		if email.ThreadID == threadID {
			email.AccountID = account.ID
			email.FolderName = folder
			thread = append(thread, email)
		}
	}
	sort.SliceStable(thread, func(i, j int) bool {
		ti, tj := parsedThreadDate(thread[i].Date), parsedThreadDate(thread[j].Date)
		if ti.Equal(tj) {
			return thread[i].UID < thread[j].UID
		}
		return ti.Before(tj)
	})
	return thread, nil
}

func (s *EmailService) decorateDetailThread(accountID, folder string, detail *EmailDetail) {
	if detail == nil {
		return
	}
	emails, err := s.db.LoadEmails(accountID, folder)
	if err != nil {
		return
	}
	for _, email := range emails {
		if email.UID != detail.UID {
			continue
		}
		if len(email.InReplyTo) > 0 {
			detail.InReplyTo = email.InReplyTo
		}
		if len(email.References) > 0 {
			detail.References = email.References
		}
		detail.ThreadID = email.ThreadID
		detail.ThreadCount = email.ThreadCount
		detail.ThreadUnread = email.ThreadUnread
		return
	}
}
