//go:build darwin

package main

import "github.com/ebitengine/purego"

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
