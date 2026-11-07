package services

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestAccountFolderSettingsRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	svc := &ConfigService{}
	account := EmailAccount{
		ID:    "folder-settings",
		Name:  "Work",
		Email: "work@example.com",
		FolderOrder: []string{"INBOX", "Sent Messages", "Junk"},
		FolderSettings: []AccountFolderSetting{
			{
				Name:        "INBOX",
				DisplayName: "收件箱",
				Icon:        "ri:inbox-2-line",
			},
			{
				Name:   "Projects/Archive",
				Hidden: true,
			},
		},
	}
	if err := svc.SaveAccounts([]EmailAccount{account}); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	accounts, err := svc.GetAccounts()
	if err != nil {
		t.Fatalf("GetAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("accounts = %d, want 1", len(accounts))
	}
	got := accounts[0].FolderSettings
	if len(got) != 2 {
		t.Fatalf("folder settings = %#v, want 2 entries", got)
	}
	if got[0].Name != "INBOX" || got[0].DisplayName != "收件箱" || got[0].Icon != "ri:inbox-2-line" {
		t.Fatalf("INBOX settings = %#v", got[0])
	}
	if got[1].Name != "Projects/Archive" || !got[1].Hidden {
		t.Fatalf("archive settings = %#v", got[1])
	}
	if want := []string{"INBOX", "Sent Messages", "Junk"}; !slices.Equal(accounts[0].FolderOrder, want) {
		t.Fatalf("folder order = %#v, want %v", accounts[0].FolderOrder, want)
	}

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(raw)
	for _, want := range []string{"folder_settings:", "folder_order:", "name: INBOX", "display_name: 收件箱", "icon: ri:inbox-2-line", "hidden: true"} {
		if !strings.Contains(text, want) {
			t.Errorf("config.yaml is missing %q:\n%s", want, filepath.Base(path))
		}
	}
}
