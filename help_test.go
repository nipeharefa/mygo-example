package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestHelpViewRendersSections(t *testing.T) {
	a := &app{version: "0.2.0"}
	tt := ui.NewTester(a.helpView, 640, 620)
	for _, want := range []string{
		"Help", "About", "mygo-demo · 0.2.0", "Keyboard", "Go live", "NDI output",
		stageSourceName, "Sample data", demoUsername,
	} {
		if !tt.HasText(want) {
			t.Fatalf("help is missing %q: %q", want, tt.Texts())
		}
	}
}

func TestHelpViewWithoutAVersion(t *testing.T) {
	a := &app{}
	tt := ui.NewTester(a.helpView, 640, 620)
	if !tt.HasText("development build") {
		t.Fatalf("help did not label the unversioned build: %q", tt.Texts())
	}
}
