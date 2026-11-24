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

// slideMetrics are the sizes a slide draws at: its text and the padding
// around it. The room sees the words alone: no part label, no song caption.
type slideMetrics struct {
	Text, PadX, PadY float32
}

// metricsFor scales the design sizes to a canvas width and pulls the chosen
// body size into the slider's range.
func metricsFor(width float32, textSize float64) slideMetrics {
	s := stageScale(width)
	return slideMetrics{
		Text: float32(clampTextSize(textSize)) * s,
		PadX: 72 * s,
		PadY: 84 * s,
	}
}

// slideBody draws the slide's text, centered. The caller owns the
// background, padding, centering and the drag handle, so the stage, the NDI
// feed, the editor preview and the run of show's tiles differ only in the
// metrics and colors they pass. Colors are explicit because previews sit in
// a console window: they must look like the stage without changing the
// window's theme.
func slideBody(c *ui.Context, s slide, m slideMetrics, t *ui.Theme) {
	ui.Text(c, s.Text).FontSize(m.Text).LineHeight(1.32).
		TextAlign(ui.Center).TextColor(t.Text)
}
