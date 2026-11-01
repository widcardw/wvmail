package services

import (
	"sort"
	"time"
)

// Mail-list ordering.
//
// Cached dates are RFC3339 strings, and RFC3339 keeps the sender's own UTC
// offset — `2026-10-24T21:32:01+08:00` sits next to `2026-10-24T14:51:46Z` in the
// same column. Comparing those two strings compares the clock face, not the
// instant, so the +08:00 message reads as nine hours newer than the Z one even
// when it arrived an hour earlier: a 21:32 mail from a Chinese sender outranks a
// 22:51 mail whose sender wrote UTC, and stays glued to the top of the list for
// the rest of the day. Every list therefore orders on the parsed instant.

const (
	earliestPlausibleEmailYear = 1970
	maxEmailFutureSkew         = 24 * time.Hour
)

// parseEmailTime parses a cached date string. The bool reports whether it held a
// real timestamp: rows whose message carried no Date header are stored empty, and
// those must sort as the oldest instead of silently becoming 1970 and beating
// everything else in a comparison.
func parseEmailTime(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	// RFC3339Nano is a superset of RFC3339, but Format never emits fractions, so
	// try the plain layout first and only fall back for hand-edited caches.
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// plausibleEmailTime rejects timestamps that cannot be trusted as a message
// date. Some senders put absurd values in Date (one real Gmail notification said
// 2612); falling back to the server's InternalDate is more useful than surfacing
// a date centuries in the future.
func plausibleEmailTime(t, now time.Time) bool {
	if t.IsZero() || t.Year() < earliestPlausibleEmailYear {
		return false
	}
	return !t.After(now.Add(maxEmailFutureSkew))
}

// resolveEmailDate prefers the sender's Date header and falls back to the
// server arrival time when that header is missing or implausible.
func resolveEmailDate(envelopeDate, internalDate, now time.Time) string {
	for _, candidate := range []time.Time{envelopeDate, internalDate} {
		if plausibleEmailTime(candidate, now) {
			return candidate.UTC().Format(time.RFC3339)
		}
	}
	return ""
}

// sanitizeCachedEmailDate keeps old cache rows from continuing to show a date
// that is now known to be implausible. A later server refresh can replace the
// empty value with the corrected InternalDate.
func sanitizeCachedEmailDate(raw string, now time.Time) string {
	t, ok := parseEmailTime(raw)
	if !ok || !plausibleEmailTime(t, now) {
		return ""
	}
	return raw
}

// newestFirst reports whether a belongs above b in a mail list.
//
// Same instant falls back to the higher UID: a batch of notifications can share
// one Date header to the second, and within it the later arrival is the newer
// one. Undated rows sink to the bottom together.
func newestFirst(a, b EmailSummary) bool {
	aTime, aOK := parseEmailTime(a.Date)
	bTime, bOK := parseEmailTime(b.Date)
	switch {
	case aOK && bOK:
		if !aTime.Equal(bTime) {
			return aTime.After(bTime)
		}
	case aOK != bOK:
		return aOK
	}
	return a.UID > b.UID
}

// sortEmailsNewestFirst orders a list in place, newest first.
func sortEmailsNewestFirst(emails []EmailSummary) {
	sort.SliceStable(emails, func(i, j int) bool {
		return newestFirst(emails[i], emails[j])
	})
}
