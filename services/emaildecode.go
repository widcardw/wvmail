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
// Prose must never be rewritten, so a candidate has to pass two gates:
//  1. cheap: an encoded `=` (`=3D`, which any encoded HTML body contains), or a soft line
//     break, or at least three `=XX` escapes;
//  2. the decoded bytes must be *text* — no control characters beyond tab/newline/CR.
//     `a=1f b=2c c=3d` gates in on three escapes and then fails here, because it decodes
//     to a unit separator, while every real body decodes to printable text or CJK bytes.
func tryQuotedPrintable(raw []byte) ([]byte, bool) {
	sample := raw
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	if !bytes.Contains(sample, []byte("=")) {
		return nil, false
	}
	escapes := len(qpEscapeRE.FindAllIndex(sample, 4))
	encodedEquals := bytes.Contains(sample, []byte("=3D")) || bytes.Contains(sample, []byte("=3d"))
	if escapes == 0 || !(encodedEquals || escapes >= 3 || qpSoftBreakRE.Match(sample)) {
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
			slog.Debug("quoted-printable decode stopped early",
				"decoded", len(decoded), "raw", len(raw), "error", err)
			if len(decoded) == 0 {
				decoded = raw
			}
		}
	default:
		// 7bit, 8bit, binary (or nothing advertised) — normally nothing to decode,
		// but decode it anyway when it clearly is quoted-printable.
		if healed, ok := tryQuotedPrintable(raw); ok {
			decoded = healed
			break
		}
		decoded = raw
	}

	// Step 2: decode charset to UTF-8
	if s, ok := decodeCharset(decoded, cs); ok {
		return s
	}

	return string(decoded)
}

// decodeCharset converts body bytes to UTF-8 using the charset declared in the
// MIME part. It reports false when nothing could be applied.
//
// Fallback ladder, each step only running when the previous one did not apply:
//  1. the declared charset, through the charset table (UTF-8/US-ASCII pass
//     through untouched);
//  2. bytes that are already valid UTF-8 are kept as they are, even when the
//     declared charset is unknown or missing;
//  3. GB18030 as a last resort for bytes that are not valid UTF-8: Chinese mail
//     routinely arrives as GBK/GB2312 with a missing or wrong charset
//     parameter, and replacement characters are worse than this heuristic.
func decodeCharset(raw []byte, cs string) (string, bool) {
	cs = strings.TrimSpace(cs)
	if cs != "" && !strings.EqualFold(cs, "utf-8") && !strings.EqualFold(cs, "us-ascii") {
		if reader, err := charset.Reader(cs, strings.NewReader(string(raw))); err == nil {
			if utf8bytes, err := io.ReadAll(reader); err == nil {
				return string(utf8bytes), true
			}
		}
	}
	if utf8.Valid(raw) {
		return string(raw), true
	}
	if converted, err := io.ReadAll(simplifiedchinese.GB18030.NewDecoder().Reader(strings.NewReader(string(raw)))); err == nil {
		return string(converted), true
	}
	return "", false
}
