package main

import "github.com/egoist/mygo/ui"

// altarscreen is the app's design system: a warm paper console, a dark stage,
// and exactly one beacon color that means "this is what people see". Every
// literal color in the app lives in this file; views read the theme.
//
// The system follows the system appearance but its own light/dark pairs stay
// warm, so the app never turns plain gray, and it never covers the user's
// accent, contrast or text scale: consoleTheme applies Preferences last.

const (
	// stageBackgroundHex is the output window's background. WindowOptions
	// takes a CSS color, so the window never flashes white before the
	// first frame. It is strayLight at its darkest.
	stageBackgroundHex = "#0b0a09"

	// beacon is the amber that fills a primary button and the dot in the
	// brand row. beaconText sits on it in both appearances: near black,
	// because white on #f59e0b does not reach 4.5:1.
	beaconDark    = "#f59e0b"
	beaconLight   = "#9f4407"
	beaconText    = "#1a1208"
	beaconOnDark  = "#2a1d08" // pressed/selected fill on a dark surface
	beaconOnLight = "#f4e6d0"

	// onAir is the copper red of anything that is on the output right now:
	// the LIVE badge, the on-air row, the stage card's frame. Red for the
	// room, amber for the controls.
	onAirDark  = "#e8603c"
	onAirLight = "#b4341a"

	// light console: warm paper.
	paper       = "#f7f1e6"
	paperRaised = "#fffaf1"
	paperSunken = "#efe6d6"
	paperInner  = "#e9dfcc"
	hairline    = "#ddd0ba"
	ink         = "#1f1a14"
	inkMuted    = "#6b6152"

	// dark console: lamp-lit desk, not a black void.
	desk         = "#15120f"
	deskRaised   = "#1f1b16"
	deskSunken   = "#100e0b"
	deskInner    = "#2a251e"
	deskLine     = "#3a332a"
	deskInk      = "#f3ecdf"
	deskInkMuted = "#a2977f"

	// stage: stray light on a dark room.
	strayLight    = "#0b0a09"
	strayRaised   = "#16130f"
	strayLine     = "#2b251d"
	strayInk      = "#f6f1e8"
	strayInkMuted = "#a49a8b"

	// status colors, warmed so they sit in the palette.
	okDark, okLight         = "#4fb872", "#2f7a44"
	warnDark, warnLight     = "#e5a53c", "#8a5a10"
	dangerDark, dangerLight = "#ef6a55", "#a8321f"
)

// consoleTheme recolors the default light or dark theme into the warm
// paper-and-amber console. It reads c.Theme() so the system appearance,
// text scale and contrast preferences still apply, and every view pass must
// set it because themes reset between frames.
func consoleTheme(c *ui.Context) *ui.Theme {
	t := consoleThemeFor(c.Theme().Dark)
	return t.WithPreferences(c.Preferences())
}

// stageTheme is the output's theme: always dark, whatever the system
// appearance is, because a projector in a dim room should not be white.
func stageTheme() *ui.Theme {
	t := *ui.DarkTheme()
	t.Background = ui.Hex(strayLight)
	t.Surface = ui.Hex(strayRaised)
	t.SurfaceHover = ui.Hex("#1d1914")
	t.SurfacePressed = ui.Hex("#241f18")
	t.Border = ui.Hex(strayLine)
	t.Text = ui.Hex(strayInk)
	t.TextMuted = ui.Hex(strayInkMuted)
	t.Accent = ui.Hex(beaconDark)
	t.AccentText = ui.Hex(beaconText)
	t.Success = ui.Hex(okDark)
	t.Warning = ui.Hex(warnDark)
	t.Danger = ui.Hex(dangerDark)
	t.Radius = 8
	return t.WithAccent(t.Accent)
}

// onAirColor is the copper that marks what the room is showing, in the
// console theme's appearance.
func onAirColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.Hex(onAirDark)
	}
	return ui.Hex(onAirLight)
}

// hairlines is the color of the rules that separate zones of the console:
// lighter than a control border, so structure does not read as controls.
func hairlines(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.Hex("#37312a")
	}
	return ui.Hex("#e6dccb")
}

// sunken is the well a list or a row sits in, so raised cards stand out
// from it.
func sunken(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.Hex(deskSunken)
	}
	return ui.Hex(paperSunken)
}

// raisedRow is a row drawn on top of empty desk, rather than in the run of
// show's well: library rows and the deck rows both use it.
func raisedRow(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.Hex("#1c1813")
	}
	return ui.Hex(paperRaised)
}

// rowHover fills a row under the pointer.
func rowHover(t *ui.Theme) ui.Color {
	return t.SurfaceHover
}

// rowSelected fills the row the operator is working on: a translucent
// accent, so it stays a tint of the system accent the user chose.
func rowSelected(t *ui.Theme) ui.Color {
	// A warm wash rather than the accent fill: the accent is reserved for
	// controls, and a filled row would swallow its own badges. In the dark
	// it has to sit clearly above the well it lies in, or the row reads as
	// empty.
	if t.Dark {
		return ui.Hex("#33281a")
	}
	return ui.Hex("#f2e4c8")
}

// rowSelectedEdge is the outline of the selected row, so the selection still
// reads for a color-blind operator and in high contrast.
func rowSelectedEdge(t *ui.Theme) ui.Color {
	return t.Accent.Alpha(0.55)
}

// glow is the warm halo under a card that is on air. It replaces a plain
// border rather than adding to it, so nothing gets busier.
func glow(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.Hex(onAirDark).Alpha(0.28)
	}
	return ui.Hex(onAirLight).Alpha(0.18)
}

// inkOnStage is text on a stage-colored surface, for the cards the console
// draws in the room's own colors.
func inkOnStage() ui.Color {
	return ui.Hex(strayInk)
}

// mutedOnStage is secondary text on a stage-colored surface.
func mutedOnStage() ui.Color {
	return ui.Hex(strayInkMuted)
}

// stageCardBorder is the still card's frame. The card on air swaps it for
// the on-air color.
func stageCardBorder() ui.Color {
	return ui.Hex(strayLine)
}

// beaconOnStage is the eyebrow color inside a stage-colored card.
func beaconOnStage() ui.Color {
	return ui.Hex(beaconDark)
}

// stageSurface is the fill of a stage-colored card.
func stageSurface() ui.Color {
	return ui.Hex(strayLight)
}

// dimOnStage is the blacked-out frame's fill.
func dimOnStage() ui.Color {
	return ui.Hex(stageBackgroundHex)
}

// innerWell is a recessed strip inside a raised card, as the stage panel's
// transport row.
func innerWell(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.Hex(deskInner)
	}
	return ui.Hex(paperInner)
}

// consoleThemeFor builds the console theme in a given appearance without a
// context, for tests and for callers that have no theme to read. The views
// use consoleTheme, which follows the system's appearance instead.
func consoleThemeFor(dark bool) *ui.Theme {
	base := ui.LightTheme()
	if dark {
		base = ui.DarkTheme()
	}
	t := *base
	if t.Dark {
		t.Background = ui.Hex(desk)
		t.Surface = ui.Hex(deskRaised)
		t.SurfaceHover = ui.Hex("#27221b")
		t.SurfacePressed = ui.Hex("#302a22")
		t.Border = ui.Hex(deskLine)
		t.Text = ui.Hex(deskInk)
		t.TextMuted = ui.Hex(deskInkMuted)
		t.Accent = ui.Hex(beaconDark)
		t.AccentText = ui.Hex(beaconText)
		t.Selection = ui.Hex(beaconOnDark)
		t.Success = ui.Hex(okDark)
		t.Warning = ui.Hex(warnDark)
		t.Danger = ui.Hex(dangerDark)
	} else {
		t.Background = ui.Hex(paper)
		t.Surface = ui.Hex(paperRaised)
		t.SurfaceHover = ui.Hex("#f6efe1")
		t.SurfacePressed = ui.Hex(paperSunken)
		t.Border = ui.Hex(hairline)
		t.Text = ui.Hex(ink)
		t.TextMuted = ui.Hex(inkMuted)
		t.Accent = ui.Hex(beaconLight)
		t.AccentText = ui.Hex(beaconText)
		t.Selection = ui.Hex(beaconOnLight)
		t.Success = ui.Hex(okLight)
		t.Warning = ui.Hex(warnLight)
		t.Danger = ui.Hex(dangerLight)
	}
	t.Radius = 10
	accent := t.Accent
	return t.WithAccent(accent)
}
