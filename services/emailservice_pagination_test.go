package services

import "testing"

// Paging beyond the first page must stay on the cache whenever it can fill the
// page: falling through to the server costs a full IMAP connect+login+fetch per
// flip (~0.5s), which is what made page 2 feel slow.
func TestCacheCoversPage(t *testing.T) {
	cases := []struct {
		name                  string
		page, pageSize, total int
		cached                int
		want                  bool
	}{
		{"page 2 fully cached", 2, 20, 100, 40, true},
		{"page 2 one row short", 2, 20, 100, 39, false},
		{"page 2 exactly at the boundary", 2, 20, 40, 40, true},
		{"page 3 fully cached", 3, 20, 100, 60, true},
		{"last page needs only the remaining rows", 3, 20, 55, 55, true},
		{"last page short by one", 3, 20, 55, 54, false},
		{"page past the end still needs the whole folder", 9, 20, 55, 55, true},
		{"empty cache never covers", 1, 20, 100, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cacheCoversPage(tc.page, tc.pageSize, tc.total, tc.cached)
			if got != tc.want {
				t.Errorf("cacheCoversPage(page=%d, size=%d, total=%d, cached=%d) = %v, want %v",
					tc.page, tc.pageSize, tc.total, tc.cached, got, tc.want)
			}
		})
	}
}

// The cache is ordered newest-first, so slicing it produces the same page the
// server would.
func TestPaginateSlicesCachedOrder(t *testing.T) {
	summaries := []EmailSummary{{UID: 5}, {UID: 4}, {UID: 3}, {UID: 2}, {UID: 1}}
	service := &EmailService{}

	page2 := service.paginate(summaries, 2, 2)
	if len(page2) != 2 || page2[0].UID != 3 || page2[1].UID != 2 {
		t.Fatalf("page 2 = %+v, want UID 3 then 2", page2)
	}
	last := service.paginate(summaries, 3, 2)
	if len(last) != 1 || last[0].UID != 1 {
		t.Fatalf("last page = %+v, want UID 1", last)
	}
	if got := service.paginate(summaries, 4, 2); len(got) != 0 {
		t.Fatalf("page past the end = %+v, want empty", got)
	}
}
