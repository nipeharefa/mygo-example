//go:build !darwin

package main

import (
	"errors"

	"github.com/egoist/mygo"
)

// ndiStream keeps the app's state and view compiling off macOS; only the
// darwin build has a sender behind it.
type ndiStream struct{}

// Stop does nothing: no sender was started.
func (s *ndiStream) Stop() {}

// startNDISender explains that NDI output is a macOS feature.
func startNDISender(*mygo.Window) (*ndiStream, error) {
	return nil, errors.New("NDI output is only available on macOS")
}
