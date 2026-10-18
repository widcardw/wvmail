package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteOriginalEmailPreservesRawBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "message.eml")
	want := []byte("From: sender@example.com\r\nContent-Type: text/html; charset=gbk\r\n\r\n\xD6\xD0\xCE\xC4\r\n")

	if err := writeOriginalEmail(path, want); err != nil {
		t.Fatalf("writeOriginalEmail: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("exported bytes differ from original:\n got %v\nwant %v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Errorf("export permissions = %04o; want no group/other access", info.Mode().Perm())
	}
}

func TestWriteOriginalEmailRejectsMissingPathOrEmptyMessage(t *testing.T) {
	if err := writeOriginalEmail("", []byte("message")); err == nil {
		t.Fatal("expected empty-path error")
	}
	if err := writeOriginalEmail(filepath.Join(t.TempDir(), "empty.eml"), nil); err == nil {
		t.Fatal("expected empty-message error")
	}
}

func TestOriginalEmailFilename(t *testing.T) {
	cases := []struct {
		subject string
		want    string
	}{
		{subject: "OneDrive 分享：文件更新", want: "OneDrive 分享：文件更新.eml"},
		{subject: "A/B\\C: report", want: "A_B_C_ report.eml"},
		{subject: "", want: "email.eml"},
		{subject: "already.EML", want: "already.EML"},
	}
	for _, tc := range cases {
		if got := originalEmailFilename(tc.subject); got != tc.want {
			t.Errorf("originalEmailFilename(%q) = %q; want %q", tc.subject, got, tc.want)
		}
	}
}
