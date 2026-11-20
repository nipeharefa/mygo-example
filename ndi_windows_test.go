//go:build windows

package main

import (
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"unsafe"
)

// The NDI runtime reads these structs over the C ABI, so the offsets must
// stay what Processing.NDI.structs.h declares. This test is the only place
// that can catch a layout slip before the first frame reaches OBS.
func TestNDIFrameLayout(t *testing.T) {
	frame := ndiVideoFrame{}
	offsets := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"xres", unsafe.Offsetof(frame.xres), 0},
		{"yres", unsafe.Offsetof(frame.yres), 4},
		{"fourCC", unsafe.Offsetof(frame.fourCC), 8},
		{"frameRateN", unsafe.Offsetof(frame.frameRateN), 12},
		{"frameRateD", unsafe.Offsetof(frame.frameRateD), 16},
		{"pictureAspectRatio", unsafe.Offsetof(frame.pictureAspectRatio), 20},
		{"frameFormatType", unsafe.Offsetof(frame.frameFormatType), 24},
		{"timecode", unsafe.Offsetof(frame.timecode), 32},
		{"data", unsafe.Offsetof(frame.data), 40},
		{"lineStride", unsafe.Offsetof(frame.lineStride), 48},
		{"metadata", unsafe.Offsetof(frame.metadata), 56},
		{"timestamp", unsafe.Offsetof(frame.timestamp), 64},
	}
	for _, o := range offsets {
		if o.got != o.want {
			t.Errorf("ndiVideoFrame.%s is at offset %d, NDI expects %d", o.name, o.got, o.want)
		}
	}
	if size := unsafe.Sizeof(frame); size != 72 {
		t.Errorf("ndiVideoFrame is %d bytes, NDI expects 72", size)
	}

	create := ndiSendCreate{}
	if got, want := unsafe.Offsetof(create.groups), uintptr(8); got != want {
		t.Errorf("ndiSendCreate.groups is at offset %d, NDI expects %d", got, want)
	}
	if got, want := unsafe.Offsetof(create.clockVideo), uintptr(16); got != want {
		t.Errorf("ndiSendCreate.clockVideo is at offset %d, NDI expects %d", got, want)
	}
	if got, want := unsafe.Sizeof(create), uintptr(24); got != want {
		t.Errorf("ndiSendCreate is %d bytes, NDI expects %d", got, want)
	}
}

// The standalone NDI Runtime and NDI Tools install into different folders,
// and a machine may have only one of them. Missing the runtime case means
// the app claims NDI is not installed on a machine that has it.
func TestNDICandidatesCoverBothInstallers(t *testing.T) {
	candidates := ndiLibraryCandidates()
	for _, want := range []string{
		filepath.Join(`C:\Program Files`, "NDI", "NDI 6 Runtime", ndiDLLName()),
		filepath.Join(`C:\Program Files`, "NDI", "NDI 6 Tools", "Runtime", ndiDLLName()),
		filepath.Join(`C:\Program Files`, "NDI", "NDI 5 Runtime", ndiDLLName()),
	} {
		if !slices.Contains(candidates, want) {
			t.Errorf("the candidate list has no %s; it has %v", want, candidates)
		}
	}
	if last := candidates[len(candidates)-1]; last != ndiDLLName() {
		t.Errorf("the bare DLL name should be tried last, after the install folders; got %q", last)
	}
}

// A DLL that is not an NDI runtime must be reported as such, not crash and
// not be mistaken for a working one.
func TestNDIRejectsForeignLibrary(t *testing.T) {
	handle, err := syscall.LoadLibrary("kernel32.dll")
	if err != nil {
		t.Fatalf("loading kernel32: %v", err)
	}
	defer syscall.FreeLibrary(handle)
	if _, err := bindNDI(uintptr(handle)); err == nil {
		t.Fatal("a library without NDI entry points should not resolve")
	}
}

// A window handle of zero is the state before the first frame, and it must
// fail as an error rather than as a crash.
func TestNDIStartWithoutWindow(t *testing.T) {
	if _, err := startNDISender(nil); err == nil {
		t.Fatal("starting the sender without a window should fail")
	}
}
