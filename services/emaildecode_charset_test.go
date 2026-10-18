package services

import (
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestDecodeBodyPartTrustsUTF8HTMLDeclarationOverWrongGBK(t *testing.T) {
	want := "将在文件删除后 30 天从 OneDrive 回收站中永久删除它们"
	raw := []byte(`<meta http-equiv="Content-Type" content="text/html; charset=utf-8"><p>` + want + `</p>`)

	got := decodeBodyPart(raw, "8bit", "gbk")
	if !strings.Contains(got, want) {
		t.Fatalf("UTF-8 HTML was decoded using the wrong MIME charset: got %q", got)
	}
	if strings.ContainsRune(got, '\uFFFD') {
		t.Fatalf("UTF-8 HTML contains replacement characters: %q", got)
	}
}

func TestDecodeBodyPartStillDecodesDeclaredGBK(t *testing.T) {
	const want = "中文邮件"
	raw, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(want))
	if err != nil {
		t.Fatalf("encode GBK fixture: %v", err)
	}
	if got := decodeBodyPart(raw, "8bit", "gbk"); got != want {
		t.Fatalf("decodeBodyPart(GBK) = %q; want %q", got, want)
	}
}

func TestCachedHTMLHasCharsetCorruption(t *testing.T) {
	withReplacement := &EmailDetail{
		BodyHTML: `<meta http-equiv="Content-Type" content="text/html; charset=utf-8">` + strings.Repeat("\uFFFD", 3),
	}
	if !cachedHTMLHasCharsetCorruption(withReplacement) {
		t.Fatal("expected UTF-8 HTML with several replacement characters to be refetched")
	}
	withoutDeclaration := &EmailDetail{BodyHTML: strings.Repeat("\uFFFD", 3)}
	if cachedHTMLHasCharsetCorruption(withoutDeclaration) {
		t.Fatal("HTML without an explicit UTF-8 declaration should not trigger this repair")
	}
	fewReplacements := &EmailDetail{
		BodyHTML: `<meta charset="utf-8">` + "\uFFFD",
	}
	if cachedHTMLHasCharsetCorruption(fewReplacements) {
		t.Fatal("a single replacement character should not trigger a network refetch")
	}
}
