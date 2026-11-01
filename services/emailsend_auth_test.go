package services

import (
	"testing"

	"github.com/emersion/go-sasl"
)

func TestSMTPAuthFromSASLCompletionDoesNotCallSASLNext(t *testing.T) {
	auth := smtpAuthFromSASL(sasl.NewPlainClient("", "user@example.com", "secret"))

	response, err := auth.Next([]byte("235 2.7.0 Authentication successful"), false)
	if err != nil {
		t.Fatalf("completion callback returned error: %v", err)
	}
	if response != nil {
		t.Fatalf("completion callback returned response %q, want nil", response)
	}
}

func TestSMTPAuthFromSASLForwardsRealChallenges(t *testing.T) {
	auth := smtpAuthFromSASL(xoauth2Client{username: "user@example.com", token: "token"})

	_, err := auth.Next([]byte(`{"status":"401"}`), true)
	if err == nil {
		t.Fatal("real challenge was not forwarded to the SASL client")
	}
}
