//go:build !darwin && !linux && !windows

package main

// ndiRenderSize is the size the feed would render at: the size asked for,
// since a build without a sender has no HD cap to apply.
func ndiRenderSize(w, h int) (int, int) {
	return ndiFrameSize(w, h)
}

// ndiFrameSize keeps the size asked for.
func ndiFrameSize(w, h int) (int, int) { return w, h }

// stageSourceFor stands in for the source a stream would draw with: this
// build has no sender, so there is nothing for it to draw into.
func (a *app) stageSourceFor(stop chan struct{}) interface{} { return nil }
