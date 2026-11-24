package main

import "github.com/egoist/mygo/ui"

// The output is designed at 1280x720 and every size below is in those stage
// pixels. A canvas of another width multiplies them by width/1280, so the
// preview, the windowed stage and the NDI feed draw the same slide.
const (
	stageDesignWidth  = 1280
	stageDesignHeight = 720
)

// stageScale is how much bigger or smaller a canvas is than the design.
func stageScale(width float32) float32 {
	if width <= 0 {
		return 1
	}
	return width / stageDesignWidth
}

// slideMetrics are the sizes a slide body draws at: eyebrow, letter spacing,
// body text, caption and the padding around them.
type slideMetrics struct {
	Eyebrow, Letter, Text, Caption, PadX, PadY float32
}

// metricsFor scales the design sizes to a canvas width and pulls the chosen
// body size into the slider's range.
func metricsFor(width float32, textSize float64) slideMetrics {
	s := stageScale(width)
	size := float32(clampTextSize(textSize))
	return slideMetrics{
		Eyebrow: 15 * s,
		Letter:  2.6 * s,
		Text:    size * s,
		Caption: 17 * s,
		PadX:    72 * s,
		PadY:    84 * s,
	}
}

// slideBody draws the three lines of a slide. The caller owns the background,
// padding, centering and the drag handle, so the stage and the editor preview
// differ only in the metrics and colors they pass. Colors are explicit because
// the editor preview sits in a console window: it must look like the stage
// without changing the window's theme.
func slideBody(c *ui.Context, s slide, m slideMetrics, t *ui.Theme) {
	// The eyebrow is a label the room reads once, so it is set small and
	// spaced: it names the verse without competing with it.
	ui.Text(c, s.eyebrow()).FontSize(m.Eyebrow).FontWeight(700).LetterSpacing(m.Letter).
		TextColor(t.Accent).TextAlign(ui.Center).SingleLine()
	// The block hangs a little above center so the caption has air under the
	// last line instead of touching it.
	ui.Column(c).Padding(0, m.PadY*0.22).Children(func() {
		ui.Text(c, s.Text).FontSize(m.Text).LineHeight(1.32).TextAlign(ui.Center).TextColor(t.Text)
	})
	ui.Text(c, s.caption()).FontSize(m.Caption).TextColor(t.TextMuted).TextAlign(ui.Center).
		Padding(0, m.PadY*0.34, 0, 0)
}
