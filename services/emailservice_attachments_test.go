package services

import "testing"

// Embedded images belong in the body, not in the attachment list. Two things were measured
// on real cached mail and drive the rules below: a plain-text-ish `.log` part in a sent
// message carries a Content-ID too (so "inline" alone is not enough to hide a file), and a
// newsletter's eleven footer/social images are all inline *and* referenced.
func TestVisibleAttachments(t *testing.T) {
	pdf := AttachmentInfo{Filename: "paper.pdf", PartPath: []int{2}, IsInline: false}
	logo := AttachmentInfo{Filename: "logo.png", MimeType: "image/png", PartPath: []int{1, 1}, IsInline: true}
	signature := AttachmentInfo{Filename: "sig.png", MimeType: "image/png", PartPath: []int{1, 2}, IsInline: true}
	logFile := AttachmentInfo{
		Filename: "andromeld.log",
		MimeType: "application/octet-stream",
		PartPath: []int{4},
		IsInline: true,
	}

	t.Run("rendered inline images are dropped", func(t *testing.T) {
		got := visibleAttachments([]AttachmentInfo{logo, pdf, signature}, [][]int{{1, 1}})
		if len(got) != 2 {
			t.Fatalf("expected 2 attachments, got %d: %+v", len(got), got)
		}
		if got[0].Filename != "paper.pdf" || got[1].Filename != "sig.png" {
			t.Errorf("wrong rows survived, or their order changed: %+v", got)
		}
	})

	t.Run("an inline image nothing references stays listed", func(t *testing.T) {
		if got := visibleAttachments([]AttachmentInfo{pdf, logo}, [][]int{}); len(got) != 2 {
			t.Errorf("unreferenced inline images must stay visible, got %+v", got)
		}
	})

	t.Run("a non-image with a Content-ID stays listed", func(t *testing.T) {
		// The body cannot display it, so it is a download, not embedded content.
		if got := visibleAttachments([]AttachmentInfo{logFile}, [][]int{{4}}); len(got) != 1 {
			t.Errorf("a real file must survive even when it claims to be inline, got %+v", got)
		}
	})

	t.Run("legacy cache rows drop inline images and keep real files", func(t *testing.T) {
		// A nil `rendered` means the information is gone: substitution already happened
		// before the row was stored, so an inline image there is one the body shows.
		got := visibleAttachments([]AttachmentInfo{logo, pdf, signature, logFile}, nil)
		if len(got) != 2 || got[0].Filename != "paper.pdf" || got[1].Filename != "andromeld.log" {
			t.Errorf("expected only the real files, got %+v", got)
		}
	})

	t.Run("no attachments yields an empty, non-nil slice", func(t *testing.T) {
		got := visibleAttachments(nil, nil)
		if got == nil {
			t.Error("must be an empty slice: JSON should carry [] and not null")
		}
		if len(got) != 0 {
			t.Errorf("expected nothing, got %+v", got)
		}
	})
}

// Ordinary attachments are untouched by any of the rules above.
func TestVisibleAttachmentsKeepsPlainAttachments(t *testing.T) {
	plain := []AttachmentInfo{
		{Filename: "a.pdf", PartPath: []int{2}},
		{Filename: "b.zip", PartPath: []int{3}},
	}
	if got := visibleAttachments(plain, [][]int{{2}, {3}}); len(got) != 2 {
		t.Errorf("plain attachments must always stay, got %+v", got)
	}
}
