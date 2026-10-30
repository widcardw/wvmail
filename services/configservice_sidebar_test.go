package services

import (
	"strings"
	"testing"
)

func TestSidebarCollapseFieldsDefaultOff(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := (&ConfigService{}).GetConfig()
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if cfg.CollapseSidebarSmartFolders {
		t.Error("smart-folder links must stay expanded by default")
	}
	if cfg.CollapseSidebarUtilities {
		t.Error("utility links must stay expanded by default")
	}
}

func TestSidebarCollapseFieldsRoundTripIndependently(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	svc := &ConfigService{}
	if err := svc.SaveConfigField("collapse_sidebar_smart_folders", "true"); err != nil {
		t.Fatalf("save smart folders: %v", err)
	}

	cfg, err := svc.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig after smart-folder save: %v", err)
	}
	if !cfg.CollapseSidebarSmartFolders {
		t.Error("smart folders must be collapsed after saving true")
	}
	if cfg.CollapseSidebarUtilities {
		t.Error("saving smart folders must not collapse utilities")
	}

	if err := svc.SaveConfigField("collapse_sidebar_utilities", "true"); err != nil {
		t.Fatalf("save utilities: %v", err)
	}
	cfg, err = svc.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig after utility save: %v", err)
	}
	if !cfg.CollapseSidebarUtilities {
		t.Error("utilities must be collapsed after saving true")
	}
	if !cfg.CollapseSidebarSmartFolders {
		t.Error("saving utilities must not expand smart folders")
	}
}

func TestSidebarCollapseFieldsRejectGarbage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	for _, field := range []string{"collapse_sidebar_smart_folders", "collapse_sidebar_utilities"} {
		err := (&ConfigService{}).SaveConfigField(field, "maybe")
		if err == nil {
			t.Fatalf("%s: expected an error for a non-boolean value", field)
		}
		if !strings.Contains(err.Error(), field) {
			t.Errorf("%s: error should name the field, got %q", field, err.Error())
		}
	}
}
