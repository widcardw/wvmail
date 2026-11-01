package services

import (
	"strings"
	"testing"
)

func TestReplaceComposeImageSources(t *testing.T) {
	source := `<p>Hello</p><img src="data:image/png;base64,AAAA" data-compose-image-id="image-1" alt="chart"><img src="data:image/png;base64,BBBB" data-compose-image-id="missing">`
	images := []ComposeAttachment{{ID: "image-1", ContentID: "wvmail-image-1"}}

	got, err := replaceComposeImageSources(source, images)
	if err != nil {
		t.Fatalf("replaceComposeImageSources() error = %v", err)
	}
	if !strings.Contains(got, `src="cid:wvmail-image-1"`) {
		t.Fatalf("replacement HTML missing CID source: %s", got)
	}
	if !strings.Contains(got, `data-content-id="wvmail-image-1"`) {
		t.Fatalf("replacement HTML missing content ID: %s", got)
	}
	if strings.Contains(got, `data-compose-image-id="image-1"`) {
		t.Fatalf("replacement HTML retained matched editor attribute: %s", got)
	}
	if !strings.Contains(got, `data-compose-image-id="missing"`) {
		t.Fatalf("replacement HTML removed unmatched editor attribute: %s", got)
	}
}

func TestReplaceComposeImageSourcesLeavesPlainHTMLAlone(t *testing.T) {
	source := `<p>No images here</p>`
	got, err := replaceComposeImageSources(source, nil)
	if err != nil {
		t.Fatalf("replaceComposeImageSources() error = %v", err)
	}
	if got != source {
		t.Fatalf("replaceComposeImageSources() = %q, want unchanged %q", got, source)
	}
}
