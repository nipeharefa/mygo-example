package main

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// relativeLuminance is WCAG 2.1's definition, on sRGB channels.
func relativeLuminance(c ui.Color) float64 {
	lin := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// contrastRatio is WCAG 2.1's ratio between two opaque colors.
func contrastRatio(a, b ui.Color) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// TestThemeContrast pins the pairs the console actually draws text on: body
// and secondary text on every surface, text on the primary button, the eyebrow
// inside a stage card, and the live badge. Secondary text is small (10-11 pt),
// so it has to clear 4.5:1 rather than the 3:1 a heading may use.
func TestThemeContrast(t *testing.T) {
	tt := ui.NewTester(func(c *ui.Context) {}, 1280, 800)
	_ = tt
	for _, dark := range []bool{false, true} {
		t.Run(map[bool]string{false: "light", true: "dark"}[dark], func(t *testing.T) {
			theme := consoleThemeFor(dark)
			pairs := []struct {
				name string
				fg   ui.Color
				bg   ui.Color
			}{
				{"text on background", theme.Text, theme.Background},
				{"text on surface", theme.Text, theme.Surface},
				{"text on a hovered row", theme.Text, theme.SurfaceHover},
				{"muted on background", theme.TextMuted, theme.Background},
				{"muted on surface", theme.TextMuted, theme.Surface},
				{"muted on a raised row", theme.TextMuted, raisedRow(theme)},
				{"muted on the selected row", theme.TextMuted, rowSelected(theme)},
				{"muted on the well", theme.TextMuted, sunken(theme)},
				{"accent on background", theme.Accent, theme.Background},
				{"text on accent", theme.AccentText, theme.Accent},
				{"success on surface", theme.Success, theme.Surface},
				{"warning on surface", theme.Warning, theme.Surface},
				{"danger on surface", theme.Danger, theme.Surface},
				{"on air on surface", onAirColor(theme), theme.Surface},
			}
			for _, p := range pairs {
				if got := contrastRatio(p.fg, p.bg); got < 4.5 {
					t.Errorf("%s: %.2f:1, want at least 4.5:1", p.name, got)
				}
			}
		})
	}

	st := stageTheme()
	for _, p := range []struct {
		name string
		fg   ui.Color
		bg   ui.Color
	}{
		{"stage text", st.Text, st.Background},
		{"stage caption", st.TextMuted, st.Background},
		{"stage eyebrow", st.Accent, st.Background},
		{"card text", inkOnStage(), stageSurface()},
		{"card caption", mutedOnStage(), stageSurface()},
		{"card eyebrow", beaconOnStage(), stageSurface()},
	} {
		if got := contrastRatio(p.fg, p.bg); got < 4.5 {
			t.Errorf("%s: %.2f:1, want at least 4.5:1", p.name, got)
		}
	}
}
