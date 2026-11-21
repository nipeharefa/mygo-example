//go:build windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

// ndiDLLName is the runtime the installer leaves behind, per architecture.
func ndiDLLName() string {
	if runtime.GOARCH == "arm64" {
		return "Processing.NDI.Lib.arm64.dll"
	}
	return "Processing.NDI.Lib.x64.dll"
}

// ndiLibraryCandidates lists the places the installer may have left the
// runtime, most specific first. The bare name lets the loader search PATH,
// which is why it comes last: another program's older copy should not win.
// NDI Tools and the standalone NDI Runtime use different folder names, and
// either can be the only thing installed, so both are listed.
func ndiLibraryCandidates() []string {
	name := ndiDLLName()
	var paths []string
	for _, env := range []string{ndiRuntimeDirV6, ndiRuntimeDirV5, ndiRuntimeDirAny} {
		if dir := os.Getenv(env); dir != "" {
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	programs := os.Getenv("ProgramFiles")
	if programs == "" {
		programs = `C:\Program Files`
	}
	for _, dir := range []string{
		filepath.Join(programs, "NDI", "NDI 6 Runtime"),
		filepath.Join(programs, "NDI", "NDI 6 Tools", "Runtime"),
		filepath.Join(programs, "NDI", "NDI 5 Runtime"),
		filepath.Join(programs, "NDI", "NDI 5 Tools", "Runtime"),
		filepath.Join(programs, "NDI", "NDI 5"),
	} {
		paths = append(paths, filepath.Join(dir, name))
	}
	return append(paths, name)
}

// ndiInstallHint names the file a missing install should have.
func ndiInstallHint() string {
	return ndiDLLName() + " in the NDI install folder or on PATH"
}

// ndiCaptureFailure explains a start that drew no first frame. Rendering the
// stage needs the main thread, which is busy while the console's first frame
// is still being built.
func ndiCaptureFailure() string {
	return "the stage is not ready to draw yet; try NDI again once the console has shown a frame"
}

// openNDILibrary loads the DLL. purego has no Dlopen on Windows, so this is
// the ordinary loader.
func openNDILibrary(path string) (uintptr, error) {
	handle, err := syscall.LoadLibrary(path)
	if err != nil {
		return 0, err
	}
	return uintptr(handle), nil
}

// closeNDILibrary releases the DLL on Stop.
func closeNDILibrary(handle uintptr) {
	syscall.FreeLibrary(syscall.Handle(handle))
}

// ndiSymbol looks one entry point up by name.
func ndiSymbol(handle uintptr, name string) (uintptr, error) {
	return syscall.GetProcAddress(syscall.Handle(handle), name)
}
