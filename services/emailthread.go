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
	minDate := make([]time.Time, len(emails))
	maxDate := make([]time.Time, len(emails))
	for i := range parent {
		parent[i] = i
		minDate[i] = parsedThreadDate(emails[i].Date)
		maxDate[i] = minDate[i]
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
			minDate[ra], maxDate[ra] = mergeThreadDateSpan(
				minDate[ra], maxDate[ra], minDate[rb], maxDate[rb],
			)
		}
	}
	// Weak links are the only ones that need the 14-day window. Checking the
	// span of the whole merged group (not just the candidate pair) prevents
	// A-B and B-C from dragging A and C together months apart.
	unionWeak := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		lo, hi := mergeThreadDateSpan(minDate[ra], maxDate[ra], minDate[rb], maxDate[rb])
		if !lo.IsZero() && !hi.IsZero() && hi.Sub(lo) > weakThreadWindow {
			return
		}
		parent[rb] = ra
		minDate[ra], maxDate[ra] = lo, hi
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
				unionWeak(i, j)
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

func mergeThreadDateSpan(lo1, hi1, lo2, hi2 time.Time) (time.Time, time.Time) {
	lo, hi := lo1, hi1
	if lo.IsZero() || (!lo2.IsZero() && lo2.Before(lo)) {
		lo = lo2
	}
	if hi.IsZero() || (!hi2.IsZero() && hi2.After(hi)) {
		hi = hi2
	}
	return lo, hi
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
		// Subject + participants alone are not unique: the same two people can
		// start a fresh "Weekly note" thread months later, outside the weak
		// window, and both groups would receive the same ID. Anchor the hash to
		// the newest cached message: older pages can be filled in later without
		// changing the ID, while a genuinely new message starts a new revision.
		anchor := ""
		for i := len(indexes) - 1; i >= 0; i-- {
			if id := normalizeMessageID(emails[indexes[i]].MessageID); id != "" {
				anchor = id
				break
			}
		}
		if anchor == "" {
			newest := emails[indexes[len(indexes)-1]]
			anchor = fmt.Sprintf("uid:%s:%s:%d", newest.AccountID, newest.FolderName, newest.UID)
		}
		sum := sha1.Sum([]byte(subject + "\x00" + strings.Join(addresses, "\x00") + "\x00" + anchor))
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
