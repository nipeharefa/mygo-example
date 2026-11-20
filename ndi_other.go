//go:build !darwin

package main

import (
	"errors"

	"github.com/egoist/mygo"
)

// ndiSupported reports whether this build can publish the window; only the
// darwin build can, so the header leaves the toggle out here.
func ndiSupported() bool { return false }

// ndiStream keeps the app's state and view compiling off macOS; only the
// darwin build has a sender behind it.
type ndiStream struct{}

// Stop does nothing: no sender was started.
func (s *ndiStream) Stop() {}

// startNDISender is the safety net behind startNDI, which already returns
// early on a build without a sender.
func startNDISender(*mygo.Window) (*ndiStream, error) {
	return nil, errors.New("NDI output is only available on macOS")
}
