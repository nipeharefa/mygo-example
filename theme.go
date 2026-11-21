package main

import "github.com/egoist/mygo/ui"

// stageBackgroundHex is the output window's background. WindowOptions takes
// a CSS color, so the window never flashes white before the first frame.
const stageBackgroundHex = "#0b0a09"

// consoleTheme recolors the default light or dark theme into the warm
// paper-and-amber console. It reads c.Theme() so the system appearance,
// text scale and contrast preferences still apply, and every view pass must
// set it because themes reset between frames.
func consoleTheme(c *ui.Context) *ui.Theme {
	t := *c.Theme()
	if t.Dark {
		t.Background = ui.Hex("#191614")
		t.Surface = ui.Hex("#241f1b")
		t.SurfaceHover = ui.Hex("#2e2823")
		t.SurfacePressed = ui.Hex("#38312a")
		t.Border = ui.Hex("#3d352c")
		t.Text = ui.Hex("#f4efe6")
		t.TextMuted = ui.Hex("#a49a8b")
		t.Accent = ui.Hex("#f59e0b")
	} else {
		t.Background = ui.Hex("#fbf8f3")
		t.Surface = ui.Hex("#f3eee4")
		t.SurfaceHover = ui.Hex("#ebe4d7")
		t.SurfacePressed = ui.Hex("#e2d9c9")
		t.Border = ui.Hex("#ddd3c2")
		t.Text = ui.Hex("#211c16")
		t.TextMuted = ui.Hex("#6e6455")
		t.Accent = ui.Hex("#b45309")
	}
	t.Radius = 10
	accent := t.Accent
	return t.WithAccent(accent).WithPreferences(c.Preferences())
}

// stageTheme is the output's theme: always dark, whatever the system
// appearance is, because a projector in a dim room should not be white.
func stageTheme() *ui.Theme {
	t := *ui.DarkTheme()
	t.Background = ui.Hex("#0b0a09")
	t.Surface = ui.Hex("#161311")
	t.SurfaceHover = ui.Hex("#1d1916")
	t.SurfacePressed = ui.Hex("#241f1b")
	t.Border = ui.Hex("#2a2521")
	t.Text = ui.Hex("#f6f1e8")
	t.TextMuted = ui.Hex("#a49a8b")
	t.Accent = ui.Hex("#f59e0b")
	t.Radius = 8
	return t.WithAccent(t.Accent)
}
