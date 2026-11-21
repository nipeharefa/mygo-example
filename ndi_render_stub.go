//go:build !darwin && !linux && !windows

package main

// ndiRenderSize is the size the feed would render at: the size asked for,
// since a build without a sender has no HD cap to apply.
func ndiRenderSize(w, h int) (int, int) {
	return ndiFrameSize(w, h)
}

// ndiFrameSize keeps the size asked for.
func ndiFrameSize(w, h int) (int, int) { return w, h }

// frameSource hands the stream a source that draws nothing. There is no
// sender type to return here, so the platform file's factory shape stands
// in for it: this build only has to compile the app's own state and view.
func (a *app) frameSource(w, h int) interface{} {
	return noSource{}
}

// noSource draws no frames; there is no frame type here to hand back.
type noSource struct{}
