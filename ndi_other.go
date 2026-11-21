//go:build !darwin && !linux && !windows

package main

import (
	"errors"
	"time"

	"github.com/egoist/mygo"
)

// ndiSupported reports whether this build can publish the window; the three
// platforms with a binding can, so the toggle is hidden elsewhere.
func ndiSupported() bool { return false }

// ndiStream keeps the app's state and view compiling where no sender is
// bound; the platforms with a sender have it in ndi_sender.go.
type ndiStream struct{}

// ndiStats is what the Settings page reports about the stream.
type ndiStats struct {
	Frames, Dropped int64
	Width, Height   int
	FPS             float64
	Uptime          time.Duration
	Err             string
}

// snapshot reports nothing: no sender was started.
func (s *ndiStream) snapshot() ndiStats { return ndiStats{} }

// Stop does nothing: no sender was started.
func (s *ndiStream) Stop() {}

// startNDISender is the safety net behind startNDI, which already returns
// early on a build without a sender.
func startNDISender(*mygo.Window) (*ndiStream, error) {
	return nil, errors.New("NDI output is not supported on this platform")
}
