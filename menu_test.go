package main

import "testing"

// The menu is built without an application, so its structure can be tested
// headlessly; the click callbacks are not exercised because they open
// windows.
func TestBuildMenuItems(t *testing.T) {
	m := buildMenu(&app{})
	if m == nil {
		t.Fatal("buildMenu returned nil")
	}

	settings := m.ItemByID("settings")
	if settings == nil {
		t.Fatal("no settings item")
	}
	if settings.Accelerator != "CmdOrCtrl+," {
		t.Fatalf("settings accelerator = %q", settings.Accelerator)
	}
	if settings.Click == nil {
		t.Fatal("settings item has no click handler")
	}

	for _, id := range []string{"help", "about"} {
		item := m.ItemByID(id)
		if item == nil {
			t.Fatalf("no %s item", id)
		}
		if item.Click == nil {
			t.Fatalf("%s item has no click handler", id)
		}
	}
}
