package services

import "testing"

func TestSenderNamePersistsSeparatelyFromAccountAlias(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	svc := &ConfigService{}
	if err := svc.SaveAccounts([]EmailAccount{{
		ID:         "a",
		Name:       "工作邮箱",
		SenderName: "张三",
		Email:      "sender@example.com",
	}}); err != nil {
		t.Fatalf("SaveAccounts: %v", err)
	}

	accounts, err := svc.GetAccounts()
	if err != nil {
		t.Fatalf("GetAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("len(accounts) = %d, want 1", len(accounts))
	}
	if accounts[0].Name != "工作邮箱" {
		t.Errorf("Name = %q, want the sidebar alias", accounts[0].Name)
	}
	if accounts[0].SenderName != "张三" {
		t.Errorf("SenderName = %q, want 张三", accounts[0].SenderName)
	}
}
