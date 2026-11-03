package services

import (
	"testing"

	"github.com/emersion/go-imap/v2"
)

func TestEnvelopeAddressRendersHostAndBareMailbox(t *testing.T) {
	if got := envelopeAddress(imap.Address{Mailbox: "alice", Host: "example.com"}); got != "alice@example.com" {
		t.Fatalf("envelopeAddress(with host) = %q, want alice@example.com", got)
	}
	if got := envelopeAddress(imap.Address{Mailbox: "local"}); got != "local" {
		t.Fatalf("envelopeAddress(without host) = %q, want local", got)
	}
}

func TestEnvelopeAddressesKeepsOrderAndNil(t *testing.T) {
	if got := envelopeAddresses(nil); got != nil {
		t.Fatalf("envelopeAddresses(nil) = %#v, want nil", got)
	}
	got := envelopeAddresses([]imap.Address{
		{Mailbox: "alice", Host: "example.com"},
		{Mailbox: "bob"},
	})
	if len(got) != 2 || got[0] != "alice@example.com" || got[1] != "bob" {
		t.Fatalf("envelopeAddresses = %#v, want [alice@example.com bob]", got)
	}
}

func TestEnvelopeSenderUsesFirstFrom(t *testing.T) {
	display, name, address := envelopeSender([]imap.Address{
		{Mailbox: "alice", Host: "example.com", Name: "Alice"},
		{Mailbox: "bob", Host: "example.com"},
	})
	if display != "Alice <alice@example.com>" || name != "Alice" || address != "alice@example.com" {
		t.Fatalf("envelopeSender = (%q, %q, %q), want Alice <alice@example.com>", display, name, address)
	}
}

func TestEnvelopeSenderWithoutName(t *testing.T) {
	display, name, address := envelopeSender([]imap.Address{{Mailbox: "alice", Host: "example.com"}})
	if display != "alice@example.com" || name != "" || address != "alice@example.com" {
		t.Fatalf("envelopeSender(no name) = (%q, %q, %q), want bare address", display, name, address)
	}
}

func TestEnvelopeSenderEmpty(t *testing.T) {
	display, name, address := envelopeSender(nil)
	if display != "" || name != "" || address != "" {
		t.Fatalf("envelopeSender(nil) = (%q, %q, %q), want empty", display, name, address)
	}
}
