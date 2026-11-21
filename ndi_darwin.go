//go:build darwin

package main

import (
	"math"

	"github.com/ebitengine/purego"
)

// ndiLibraryCandidates is where the NDI runtime installs on macOS, most
// specific first; the bare name lets dyld search the usual paths.
func ndiLibraryCandidates() []string {
	return []string{
		"/usr/local/lib/libndi.dylib",
		"/Library/NDI SDK for Apple/lib/macOS/libndi.dylib",
		"libndi.dylib",
	}
}

// ndiInstallHint names the file a missing install should have.
func ndiInstallHint() string {
	return "/usr/local/lib/libndi.dylib"
}

// ndiCaptureFailure explains a start that drew no first frame. Rendering the
// stage needs the main thread, which is busy while the console's first frame
// is still being built.
func ndiCaptureFailure() string {
	return "the stage is not ready to draw yet; try NDI again once the console has shown a frame"
}

// openNDILibrary loads the NDI runtime through purego's dlopen.
func openNDILibrary(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
}

// closeNDILibrary releases the runtime on Stop.
func closeNDILibrary(handle uintptr) {
	_ = purego.Dlclose(handle)
}

// ndiSymbol looks one entry point up by name.
func ndiSymbol(handle uintptr, name string) (uintptr, error) {
	return purego.Dlsym(handle, name)
}

// ndiFrameSize fits a requested frame into the HD cap, keeping its aspect
// ratio. A size already at or below the cap is kept as it is.
func ndiFrameSize(w, h int) (int, int) {
	scale := 1.0
	if w > ndiMaxFrameWidth {
		scale = float64(ndiMaxFrameWidth) / float64(w)
	}
	if h > ndiMaxFrameHeight && float64(ndiMaxFrameHeight)/float64(h) < scale {
		scale = float64(ndiMaxFrameHeight) / float64(h)
	}
	if scale >= 1 {
		return w, h
	}
	nw, nh := int(math.Round(float64(w)*scale)), int(math.Round(float64(h)*scale))
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	// Keep the dimensions even, as video encoders prefer 4:2:0 frames.
	nw &^= 1
	nh &^= 1
	return nw, nh
}
