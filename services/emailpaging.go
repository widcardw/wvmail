package services

// Page coordinates and slicing shared by every paginated mail list.
//
// The folder list, search, and tag-filtered lists all accept (page, pageSize)
// from the frontend. They used to clamp the values and slice the rows
// independently, which is how the "page 1" rule in RefreshEmails drifted from
// the cache rule used by ListEmails. Keeping one implementation here makes the
// bounds and the empty-page shape identical everywhere.

// normalizePage returns valid 1-based page coordinates. A missing or
// non-positive value is defaulted rather than rejected: the frontend sends zero
// for "unspecified" in several call paths.
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return page, pageSize
}

// paginateSlices returns the requested page of items. It always returns a
// non-nil slice so a JSON response carries [] instead of null.
func paginateSlices[T any](items []T, page, pageSize int) []T {
	page, pageSize = normalizePage(page, pageSize)
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []T{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

// cacheCoversPage reports whether the cached rows can fill the requested page
// completely. The last page only needs the rows that exist, so a cache holding
// exactly total rows still covers it.
func cacheCoversPage(page, pageSize, total, cachedLen int) bool {
	needed := page * pageSize
	if needed > total {
		needed = total
	}
	return cachedLen >= needed
}
