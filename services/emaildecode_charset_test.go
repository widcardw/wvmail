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

func TestDecodeBodyPartFallsBackToGB18030WithoutCharset(t *testing.T) {
	const want = "中文邮件"
	raw, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(want))
	if err != nil {
		t.Fatalf("encode GBK fixture: %v", err)
	}
	if got := decodeBodyPart(raw, "8bit", ""); got != want {
		t.Fatalf("decodeBodyPart(no charset) = %q; want %q", got, want)
	}
}

// A stale `us-ascii`/`gbk` header must not beat bytes that are plainly UTF-8: Outlook
// reports us-ascii for this class of mail, and decoding UTF-8 as ASCII is harmless while
// decoding it as GBK is not.
func TestDecodeCharsetPrefersValidUTF8OverLegacyHeader(t *testing.T) {
	const want = "将在文件删除后"
	for _, cs := range []string{"gbk", "gb2312", "us-ascii", "iso-8859-1", "big5"} {
		if got, ok := decodeCharset([]byte(want), cs); !ok || got != want {
			t.Errorf("charset=%q: got %q (ok=%v); want %q", cs, got, ok, want)
		}
	}
}

func TestDecodeUTF8HTMLPreservesLiteralReplacementGlyphs(t *testing.T) {
	want := `<meta charset="utf-8"><p>将在文件删除后</p>` + strings.Repeat("\uFFFD", 31)
	got := decodeBodyPart([]byte(want), "8bit", "utf-8")
	if got != want {
		t.Fatalf("valid UTF-8 source changed during decode: replacement_count=%d, want=%d", strings.Count(got, "\uFFFD"), strings.Count(want, "\uFFFD"))
	}
	if _, changed := repairGBKMojibake(got); changed {
		t.Fatal("literal U+FFFD characters must not be treated as mojibake")
	}
}

// The OneDrive notice (uid 1958): a valid UTF-8, `8bit` HTML body whose only "escapes"
// are `content="IE=edge"` and `?LinkID=271181`. The quoted-printable heuristic used to
// fire on those, turn `=ed` into 0xED, and leave the reader with 灏嗗湪鏂囦欢 — which then
// looked exactly like UTF-8 decoded as GBK.
func TestDecodeBodyPartIgnoresHexLookingAttributeValues(t *testing.T) {
	want := "我们注意到你最近从 OneDrive 中删除了大量文件"
	raw := []byte(`<!DOCTYPE html><html><head>` +
		`<meta http-equiv="X-UA-Compatible" content="IE=edge">` +
		`<meta name="viewport" content="width=device-width, initial-scale=1.0">` +
		`</head><body><p>` + want + `</p>` +
		`<a href="https://go.microsoft.com/fwlink/?LinkID=271181">OneDrive</a>` +
		`<a href="https://go.microsoft.com/fwlink/?LinkID=857496">Privacy</a>` +
		`</body></html>`)

	got := decodeBodyPart(raw, "8bit", "utf-8")
	if got != string(raw) {
		t.Fatalf("valid UTF-8 body was rewritten: replacement=%d, got %q", strings.Count(got, "\uFFFD"), got)
	}
	if !strings.Contains(got, want) {
		t.Fatalf("Chinese text lost: %q", got)
	}
}

func TestRepairGBKMojibake(t *testing.T) {
	const correct = "将在文件删除后从回收站中永久删除它们"
	misdecoded, err := simplifiedchinese.GBK.NewDecoder().String(correct)
	if err != nil {
		t.Fatalf("make mojibake fixture: %v", err)
	}
	if misdecoded == correct {
		t.Fatal("fixture is not mojibake")
	}

	repaired, ok := repairGBKMojibake(`<meta charset="utf-8"><p>` + misdecoded + `</p>`)
	if !ok {
		t.Fatalf("failed to repair UTF-8 decoded as GBK: %q", misdecoded)
	}
	if !strings.Contains(repaired, correct) {
		t.Fatalf("repaired text still wrong: %q", repaired)
	}
	if _, changed := repairGBKMojibake(correct); changed {
		t.Fatal("normal Chinese text must not be classified as mojibake")
	}
	if _, changed := repairGBKMojibake(`<p>hello world</p>`); changed {
		t.Fatal("ASCII text must not be touched")
	}
}

func TestCachedHTMLHasCharsetCorruption(t *testing.T) {
	misdecoded, err := simplifiedchinese.GBK.NewDecoder().String("将在文件删除后")
	if err != nil {
		t.Fatalf("make mojibake fixture: %v", err)
	}
	if !cachedHTMLHasCharsetCorruption(&EmailDetail{BodyHTML: `<p>` + misdecoded + `</p>`}) {
		t.Fatalf("expected to detect GBK mojibake in cached HTML: %q", misdecoded)
	}
	healthy := &EmailDetail{BodyHTML: `<meta charset="utf-8"><p>将在文件删除后</p>` + strings.Repeat("\uFFFD", 31)}
	if cachedHTMLHasCharsetCorruption(healthy) {
		t.Fatal("literal replacement characters alone must not trigger a refetch")
	}
}
