package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The view runs without a window in tests, which click as a user would.
func TestResourceSwitching(t *testing.T) {
	a := newApp()
	tt := ui.NewTester(a.view, 1180, 720)

	// The default kind shows its first object in the table and inspector.
	if !tt.HasText("Pods") || !tt.HasText("api-7d9f8c-x2k4p") {
		t.Fatalf("the Pods pane did not show: %q", tt.Texts())
	}

	if err := tt.Click("Deployments"); err != nil {
		t.Fatal(err)
	}
	if a.kind != "deployments" {
		t.Fatalf("kind %q after clicking Deployments", a.kind)
	}
	if !tt.HasText("api-gateway") {
		t.Fatalf("the Deployments rows did not show: %q", tt.Texts())
	}

	// A new kind starts at its first object, whatever row was chosen.
	a.row = 3
	if err := tt.Click("Secrets"); err != nil {
		t.Fatal(err)
	}
	if a.kind != "secrets" {
		t.Fatalf("kind %q after clicking Secrets", a.kind)
	}
	if a.row != 0 {
		t.Errorf("row %d after switching kinds, want 0", a.row)
	}
	if !tt.HasText("regcred") || !tt.HasText("2 keys") {
		t.Fatalf("the Secrets rows did not show: %q", tt.Texts())
	}
}
