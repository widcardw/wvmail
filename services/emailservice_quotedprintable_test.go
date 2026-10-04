package services

import (
	"strings"
	"testing"
)

// Bulk mail routinely advertises `8bit` (or no Content-Transfer-Encoding at all) while
// shipping quoted-printable content: `style=3D"…"`, `=09` tabs and `=E8=AE=A1` for 计.
// Left alone the reader sees the raw escapes, so the decoder has to notice.
func TestDecodeBodyPartUnlabelledQuotedPrintable(t *testing.T) {
	// Shaped like the real message (uid 274 in the local cache): encoded `=`, a tab,
	// a soft line break and UTF-8 Chinese.
	raw := []byte("<div style=3D\"padding: 15px;\"> =09=09<div>=\r\n" +
		"=E8=AE=A1=E7=AE=97=E6=9C=BA=E7=A7=91=E5=AD=A6</div>")

	for _, enc := range []string{"", "8bit", "7bit", "binary", " 8BIT "} {
		got := decodeBodyPart(raw, enc, "utf-8")
		if !strings.Contains(got, `style="padding: 15px;"`) {
			t.Errorf("enc=%q: =3D not decoded: %q", enc, got)
		}
		if !strings.Contains(got, "计算机科学") {
			t.Errorf("enc=%q: UTF-8 escapes not decoded: %q", enc, got)
		}
		if strings.Contains(got, "=09") || strings.Contains(got, "=\r\n") {
			t.Errorf("enc=%q: escapes left behind: %q", enc, got)
		}
	}
}

// A genuine quoted-printable part keeps working (including a padded/cased header value).
func TestDecodeBodyPartLabelledQuotedPrintable(t *testing.T) {
	raw := []byte("caf=C3=A9=09tab")
	for _, enc := range []string{"quoted-printable", "QUOTED-PRINTABLE", " quoted-printable\r"} {
		if got := decodeBodyPart(raw, enc, "utf-8"); got != "café\ttab" {
			t.Errorf("enc=%q: got %q, want %q", enc, got, "café\ttab")
		}
	}
}

// Ordinary prose must never be run through the decoder: an `=` in a sentence is not an
// escape, and a base64-looking token must not be "fixed" either.
func TestDecodeBodyPartLeavesPlainTextAlone(t *testing.T) {
	cases := []string{
		"a = b",
		"If x = 42 then y = 42 and z = 42.",
		"token: YWJjZGVmZ2hpamtsbW5vcA==",
		"Version 1.2 = stable",
	}
	for _, body := range cases {
		if got := decodeBodyPart([]byte(body), "8bit", "utf-8"); got != body {
			t.Errorf("plain text was rewritten: %q -> %q", body, got)
		}
	}
}

// Bodies cached before the heuristic existed are repaired on read, and a body that is
// already decoded must be left exactly as it is.
func TestRepairCachedEncoding(t *testing.T) {
	detail := &EmailDetail{
		BodyHTML: "<div style=3D\"padding: 15px;\">=E8=AE=A1=E7=AE=97</div>",
		BodyText: "plain text, nothing encoded",
	}
	if !repairCachedEncoding(detail) {
		t.Fatal("stored quoted-printable body was not repaired")
	}
	if !strings.Contains(detail.BodyHTML, `style="padding: 15px;"`) || !strings.Contains(detail.BodyHTML, "计算") {
		t.Errorf("repaired body still wrong: %q", detail.BodyHTML)
	}
	if detail.BodyText != "plain text, nothing encoded" {
		t.Errorf("plain text body was rewritten: %q", detail.BodyText)
	}

	// Second pass: nothing left to fix, so the row is not rewritten again.
	again := &EmailDetail{BodyHTML: detail.BodyHTML, BodyText: detail.BodyText}
	if repairCachedEncoding(again) {
		t.Error("an already decoded body must not be reported as changed")
	}
}

// The real failure (Microsoft CMT mail via SZU): the part *is* declared quoted-printable
// and ends with a NUL padding byte. The streaming decoder stopped at that byte, and the
// old all-or-nothing error check threw away the ~9 KB it had already decoded.
func TestDecodeBodyPartWithTrailingNul(t *testing.T) {
	raw := append([]byte("<p style=3D\"x\">=E8=AE=A1=E7=AE=97</p>\r\n"), 0x00)
	got := decodeBodyPart(raw, "QUOTED-PRINTABLE", "UTF-8")
	if strings.Contains(got, "=E8") || !strings.Contains(got, "计算") {
		t.Errorf("trailing NUL defeated the decode: %q", got)
	}
	if strings.ContainsRune(got, 0) {
		t.Errorf("NUL byte survived into the body: %q", got)
	}
}

// A malformed escape at the very end must not cost the whole body either.
func TestDecodeBodyPartKeepsPartialQuotedPrintable(t *testing.T) {
	raw := []byte("=E8=AE=A1=E7=AE=97 body text =ZZ")
	got := decodeBodyPart(raw, "quoted-printable", "utf-8")
	if !strings.Contains(got, "计算 body text") {
		t.Errorf("decoded head was discarded: %q", got)
	}
}

// Same tolerance for a body whose CTE was not labelled at all (the heuristic path).
func TestTryQuotedPrintableWithTrailingJunk(t *testing.T) {
	raw := append([]byte("<div style=3D\"x\">=E8=AE=A1=E7=AE=97</div>"), 0x00)
	got, ok := tryQuotedPrintable(raw)
	if !ok {
		t.Fatal("NUL-padded quoted-printable body was not recognised")
	}
	if !strings.Contains(string(got), "计算") {
		t.Errorf("decoded body wrong: %q", got)
	}

	// ...but a candidate whose decode dies early is still rejected. mime/quotedprintable
	// only errors on unescaped control bytes (a bad escape like `=ZZ` is passed through
	// literally), so the early-death case is a stray control byte near the start.
	stunted := "ab\x01" + strings.Repeat("long trailing filler ", 15)
	if _, ok := tryQuotedPrintable([]byte(stunted)); ok {
		t.Error("a candidate that stops decoding after a few bytes must be rejected")
	}
}

// A candidate must both look encoded and decode to text; prose with stray `=` must not
// be rewritten, and `=XX`-looking tokens that decode to control bytes are rejected.
func TestTryQuotedPrintable(t *testing.T) {
	yes := map[string]string{
		`<div style=3D"x">a</div>`:       `<div style="x">a</div>`,
		"wrap=\nnext =E8=AE=A1":          "wrapnext 计",
		"=09=09=E8=AE=A1=E7=AE=97=0A=0D": "\t\t计算\n\r",
		"caf=C3=A9 =2B more":             "café + more",
	}
	for body, want := range yes {
		got, ok := tryQuotedPrintable([]byte(body))
		if !ok {
			t.Errorf("not detected as quoted-printable: %q", body)
			continue
		}
		if string(got) != want {
			t.Errorf("decoded %q -> %q, want %q", body, got, want)
		}
	}

	no := []string{
		"no equals sign here",
		"a = b, c = d",
		"subject =?UTF-8?B?5rWL6K+V?=",
		"already decoded: 计算机科学与技术",
		"a=1f b=2c c=3d", // three escapes, but decodes to a control byte
	}
	for _, body := range no {
		if got, ok := tryQuotedPrintable([]byte(body)); ok {
			t.Errorf("false positive on plain text: %q -> %q", body, got)
		}
	}
}
