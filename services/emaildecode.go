package services

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/emersion/go-message/charset"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// MIME/charset decoding: encoded-words in headers, and the transfer-encoding +
// charset soup of real-world body parts.

// decodeMimeHeader decodes a MIME encoded-word (=?charset?B?...?= / =?charset?Q?...?=)
// with support for Chinese character sets (GBK, GB2312, GB18030).
func decodeMimeHeader(s string) string {
	if !strings.Contains(s, "=?") {
		return s
	}
	dec := &mime.WordDecoder{
		CharsetReader: func(charset string, input io.Reader) (io.Reader, error) {
			switch strings.ToLower(charset) {
			case "gbk", "gb2312", "gb18030":
				return simplifiedchinese.GBK.NewDecoder().Reader(input), nil
			default:
				return nil, fmt.Errorf("unhandled charset: %s", charset)
			}
		},
	}
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		return s // fallback to original
	}
	return decoded
}

// quoted-printable escapes (`=3D`, `=E8`, `=09`) and the soft line break (`=` at the
// end of a line) used to detect bodies whose CTE was mislabelled.
var (
	qpEscapeRE    = regexp.MustCompile(`(?i)=[0-9a-f]{2}`)
	qpSoftBreakRE = regexp.MustCompile(`=(?:\r\n|\n)`)
)

// tryQuotedPrintable decodes bytes that were declared 7bit/8bit (or had no
// Content-Transfer-Encoding at all) but are in fact quoted-printable. Senders and
// gateways mislabel this constantly: bulk mail arrives as `8bit` while the body is full
// of `style=3D"…"` and `=09` tabs, and the reader ends up staring at raw escapes.
//
// Prose must never be rewritten, so a candidate has to pass three gates:
//  1. cheap: the body contains `=`, and at least one escape;
//  2. a *strong* signal — an encoded `=`, a soft line break, or a dense run of escapes.
//     Counting escapes alone is not enough: real HTML is full of `=` followed by two
//     hex digits by accident (`content="IE=edge"`, `?LinkID=271181`), and a OneDrive
//     notice with four such tokens in 8 KiB used to be "healed" into `0xED` bytes. A
//     quoted-printable body, by contrast, either escapes its `=` (every HTML body has
//     `style=3D`), wraps its long lines with a soft break, or spends roughly one escape
//     per three bytes on CJK text;
//  3. the decoded bytes must be *text* — no control characters beyond tab/newline/CR.
//     `a=1f b=2c c=3d` gates in on density and then fails here, because it decodes to a
//     unit separator, while every real body decodes to printable text or CJK bytes.
func tryQuotedPrintable(raw []byte) ([]byte, bool) {
	sample := raw
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	if !bytes.Contains(sample, []byte("=")) {
		return nil, false
	}
	escapes := len(qpEscapeRE.FindAllIndex(sample, -1))
	if escapes == 0 {
		return nil, false
	}
	encodedEquals := bytes.Contains(sample, []byte("=3D")) || bytes.Contains(sample, []byte("=3d"))
	dense := escapes*32 >= len(sample)
	if !encodedEquals && !dense && !qpSoftBreakRE.Match(sample) {
		return nil, false
	}

	decoded, err := io.ReadAll(quotedprintable.NewReader(bytes.NewReader(raw)))
	if len(decoded) == 0 {
		return nil, false
	}
	if err != nil {
		// Trailing junk stops the reader part-way; keep the decoded head only when it
		// clearly accounts for most of the input, so prose is never half-eaten.
		if len(decoded)*2 < len(raw) {
			return nil, false
		}
	}
	for _, b := range decoded {
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
			return nil, false
		}
	}
	return decoded, true
}

// decodeBodyPart decodes raw IMAP body section bytes based on
// Content-Transfer-Encoding and charset.
func decodeBodyPart(raw []byte, enc, cs string) string {
	if len(raw) == 0 {
		return ""
	}
	// NUL bytes are padding some senders/gateways append to a body section; they are
	// never part of the text and make both decoders below bail out mid-stream (a real
	// message: `<body=\r\n></html>=20\r\n\x00`, which cost the whole body).
	raw = bytes.ReplaceAll(raw, []byte{0}, nil)

	// Step 1: decode Content-Transfer-Encoding. The value comes straight from
	// BODYSTRUCTURE, so tolerate quoting artefacts and surrounding whitespace.
	var decoded []byte
	var err error
	switch strings.ToLower(strings.TrimSpace(enc)) {
	case "base64":
		decoded, err = base64.StdEncoding.DecodeString(string(raw))
		if err != nil {
			// Try with padded input
			s := strings.TrimRight(string(raw), "\n\r\t ")
			decoded, err = base64.StdEncoding.DecodeString(s)
			if err != nil {
				decoded = raw
			}
		}
	case "quoted-printable":
		decoded, err = io.ReadAll(quotedprintable.NewReader(bytes.NewReader(raw)))
		if err != nil {
			// The reader yields decoded bytes as it goes: a malformed escape near the
			// end leaves everything before it intact, and throwing that away is what
			// pushed raw `=E8=AE=A1` text all the way to the reader.
			slog.Debug("decode: quoted-printable decode stopped early",
				"decoded", len(decoded), "raw", len(raw), "error", err)
			if len(decoded) == 0 {
				decoded = raw
			}
		}
	default:
		// 7bit, 8bit, binary (or nothing advertised) — normally nothing to decode,
		// but decode it anyway when it clearly is quoted-printable. The healed bytes
		// only win when they read at least as cleanly as the untouched ones: a
		// mis-gated "repair" is itself the corruption.
		if healed, ok := tryQuotedPrintable(raw); ok {
			if text, ok := preferCleanerDecode(healed, raw, cs); ok {
				return text
			}
		}
		decoded = raw
	}

	// Step 2: decode charset to UTF-8
	if s, ok := decodeCharset(decoded, cs); ok {
		return s
	}

	return string(decoded)
}

// preferCleanerDecode reports whether the quoted-printable-healed bytes decode with no
// more replacement glyphs than the untouched bytes do, and returns that text. A body
// with `content="IE=edge"` heals into a stray 0xED, which no charset can read; without
// this check the damage looks like a charset problem and the body renders as mojibake.
func preferCleanerDecode(healed, raw []byte, cs string) (string, bool) {
	healedText, ok := decodeCharset(healed, cs)
	if !ok {
		return "", false
	}
	rawText, ok := decodeCharset(raw, cs)
	if !ok {
		return healedText, true
	}
	if strings.Count(healedText, "\uFFFD") > strings.Count(rawText, "\uFFFD") {
		return "", false
	}
	return healedText, true
}

// repairGBKMojibake rewrites runs of Han characters that are really UTF-8 bytes that
// were decoded as GBK back into the text they were meant to be — "灏嗗湪鏂囦欢" becomes
// "将在文件". It reports whether anything was rewritten.
//
// The test for a run is reversible: encode it as GBK and see whether those bytes are
// valid UTF-8 that reads as different Chinese. Genuine Chinese fails that test, because
// its GBK bytes are not a well-formed UTF-8 sequence. Runs shorter than six characters
// are left alone — they are too easy to match by accident.
func repairGBKMojibake(text string) (string, bool) {
	if text == "" {
		return text, false
	}
	runes := []rune(text)
	var out strings.Builder
	out.Grow(len(text))
	changed := false
	for i := 0; i < len(runes); {
		if !unicode.Is(unicode.Han, runes[i]) {
			out.WriteRune(runes[i])
			i++
			continue
		}
		j := i + 1
		for j < len(runes) && unicode.Is(unicode.Han, runes[j]) {
			j++
		}
		run := runes[i:j]
		if j-i >= 6 {
			if fixed, consumed, ok := decodeGBKMojibakeRun(run); ok {
				out.WriteString(fixed)
				// The bytes of a split character at the end of the run cannot be
				// recovered; keep those characters as they are rather than lose them.
				out.WriteString(string(run[consumed:]))
				changed = true
				i = j
				continue
			}
		}
		out.WriteString(string(run))
		i = j
	}
	if !changed {
		return text, false
	}
	return out.String(), true
}

// decodeGBKMojibakeRun reverses one mojibake run. It returns the recovered text and how
// many of the run's characters it accounts for; the rest is left to the caller.
//
// The run's GBK bytes are the original UTF-8 bytes, so the longest valid UTF-8 prefix is
// the recovered text. A trailing partial character is tolerated (the run may have been
// cut by a tag, a space or a replacement glyph), which is why the last one or two bytes
// are trimmed and retried.
func decodeGBKMojibakeRun(run []rune) (string, int, bool) {
	offsets := make([]int, len(run)+1) // byte offset of each character in `encoded`
	var encoded []byte
	for i, r := range run {
		offsets[i] = len(encoded)
		b, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(string(r)))
		if err != nil {
			return "", 0, false
		}
		encoded = append(encoded, b...)
	}
	offsets[len(run)] = len(encoded)

	for trim := 0; trim <= 2 && trim < len(encoded); trim++ {
		n := len(encoded) - trim
		if !utf8.Valid(encoded[:n]) {
			continue
		}
		recovered := string(encoded[:n])
		if recovered == string(run) {
			continue
		}
		hanCount := 0
		for _, r := range recovered {
			if unicode.Is(unicode.Han, r) {
				hanCount++
			}
		}
		if hanCount < 4 {
			continue
		}
		consumed := 0
		for consumed < len(run) && offsets[consumed] < n {
			consumed++
		}
		return recovered, consumed, true
	}
	return "", 0, false
}

// decodeCharset converts body bytes to UTF-8. The order of authority is:
//
//  1. the bytes themselves — a well-formed UTF-8 sequence is not something a legacy
//     charset produces, so valid UTF-8 wins even when BODYSTRUCTURE claims gbk or
//     us-ascii (Outlook and OneDrive mislabel UTF-8 constantly, and trusting the header
//     over the bytes is what turns a readable mail into mojibake);
//  2. the declared charset, when the bytes are not UTF-8;
//  3. GB18030 — a superset of GBK/GB2312 — for the CJK mail that declares nothing.
//
// Steps 2 and 3 are scored by how many bytes each decoder fails to map (U+FFFD), so a
// stale declaration loses to the decoder that actually fits the bytes.
func decodeCharset(raw []byte, cs string) (string, bool) {
	if len(raw) == 0 {
		return "", true
	}
	if utf8.Valid(raw) {
		return string(raw), true
	}

	best, bestScore := "", -1
	for _, name := range []string{strings.TrimSpace(cs), "gb18030"} {
		if name == "" || strings.EqualFold(name, "utf-8") || strings.EqualFold(name, "utf8") {
			continue
		}
		reader, err := charset.Reader(name, bytes.NewReader(raw))
		if err != nil {
			continue
		}
		decoded, err := io.ReadAll(reader)
		if err != nil || len(decoded) == 0 {
			continue
		}
		text := string(decoded)
		score := strings.Count(text, "\uFFFD")
		if bestScore < 0 || score < bestScore {
			best, bestScore = text, score
		}
	}
	if bestScore < 0 {
		return "", false
	}
	return best, true
}
