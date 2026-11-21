//go:build linux

package main

import (
	"os"
	"path/filepath"

	"github.com/ebitengine/purego"
)

// ndiLibraryCandidates is where the NDI runtime usually lands. The bare
// name comes after the install paths, so a copy left by another package
// does not win over the one NDI Tools put down.
func ndiLibraryCandidates() []string {
	var paths []string
	for _, env := range []string{ndiRuntimeDirV6, ndiRuntimeDirV5, ndiRuntimeDirAny} {
		if dir := os.Getenv(env); dir != "" {
			paths = append(paths, filepath.Join(dir, "libndi.so"))
		}
	}
	return append(paths,
		"libndi.so",
		"/usr/local/lib/libndi.so",
		"/usr/lib/libndi.so",
		"/usr/lib/x86_64-linux-gnu/libndi.so",
	)
}

// ndiInstallHint names the file a missing install should have.
func ndiInstallHint() string {
	return "libndi.so on the library path"
}

// ndiCaptureFailure explains a start that drew no first frame. Rendering the
// stage needs the main thread, which is busy while the console's first frame
// is still being built.
func ndiCaptureFailure() string {
	return "the stage is not ready to draw yet; try NDI again once the console has shown a frame"
}

// openNDILibrary loads the NDI runtime through the C library loader.
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
