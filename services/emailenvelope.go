package services

import "github.com/emersion/go-imap/v2"

// Envelope rendering shared by the list and detail paths.
//
// A message's From/To/Cc formatting used to be copied into emailSummaryFromBuffer
// and RefreshEmailBody. The two copies had already drifted (the detail path let
// the last From header win while the list used the first), so a provider quirk
// could be fixed in only one view. These helpers are the single source of truth.

// envelopeAddress renders an IMAP address as mailbox@host, or the bare mailbox
// when the server omitted the host.
func envelopeAddress(addr imap.Address) string {
	if addr.Host != "" {
		return addr.Mailbox + "@" + addr.Host
	}
	return addr.Mailbox
}

// envelopeAddresses renders a header's address list. A nil result is kept for an
// absent header so JSON omits it rather than emitting an empty array.
func envelopeAddresses(addrs []imap.Address) []string {
	if len(addrs) == 0 {
		return nil
	}
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, envelopeAddress(addr))
	}
	return out
}

// envelopeSender returns the combined display string, decoded name, and address
// for the first From header. RFC 9051 allows one From; using the first entry
// keeps the list and detail views consistent when a server sends more.
func envelopeSender(addrs []imap.Address) (display, name, address string) {
	if len(addrs) == 0 {
		return "", "", ""
	}
	addr := addrs[0]
	address = envelopeAddress(addr)
	name = decodeMimeHeader(addr.Name)
	if name != "" {
		display = name + " <" + address + ">"
	} else {
		display = address
	}
	return display, name, address
}
