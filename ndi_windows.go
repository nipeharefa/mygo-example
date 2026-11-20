//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"
)

const (
	printWindowFullContent = 2
	srcCopy                = 0x00CC0020
	dibRGBColors           = 0
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

// ndiCaptureFailure explains a window that gave no pixels.
func ndiCaptureFailure() string {
	return "cannot read the window's pixels"
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

// captureFrame draws the window with GDI; PrintWindow is safe to call from
// the capture goroutine.
func captureFrame(c ndiCapturer) (windowFrame, bool) {
	return c.capture()
}

// rect mirrors RECT.
type rect struct {
	left, top, right, bottom int32
}

// bitmapInfoHeader mirrors BITMAPINFOHEADER; bitmapInfo pads it with one
// palette entry, which CreateDIBSection reads anyway.
type bitmapInfoHeader struct {
	size          uint32
	width, height int32
	planes        uint16
	bitCount      uint16
	compression   uint32
	sizeImage     uint32
	xPelsPerMeter int32
	yPelsPerMeter int32
	clrUsed       uint32
	clrImportant  uint32
}

type bitmapInfo struct {
	header bitmapInfoHeader
	colors [1]uint32
}

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")

	procGetClientRect = user32.NewProc("GetClientRect")
	procGetDC         = user32.NewProc("GetDC")
	procReleaseDC     = user32.NewProc("ReleaseDC")
	procPrintWindow   = user32.NewProc("PrintWindow")

	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procBitBlt             = gdi32.NewProc("BitBlt")
)

// windowCapturer copies a window's own pixels out of Windows. It must run
// on the main thread only to create, and may then be read from the
// capture goroutine.
type windowCapturer struct {
	hwnd  uintptr
	frame windowFrame
}

// newWindowCapturer keeps the window handle; the first capture reports
// whether Windows gives us pixels at all.
func newWindowCapturer(handle uintptr) (*windowCapturer, error) {
	if handle == 0 {
		return nil, errors.New("the window has no handle yet")
	}
	return &windowCapturer{hwnd: handle}, nil
}

// capture draws the window into a bitmap and copies it out. PrintWindow
// asks Windows to render the window for us, which works while it is
// covered; BitBlt is the fallback for a window that refuses.
func (c *windowCapturer) capture() (windowFrame, bool) {
	var r rect
	if ok, _, _ := procGetClientRect.Call(c.hwnd, uintptr(unsafe.Pointer(&r))); ok == 0 {
		return windowFrame{}, false
	}
	w, h := int(r.right-r.left), int(r.bottom-r.top)
	if w <= 0 || h <= 0 {
		return windowFrame{}, false
	}
	dc, _, _ := procGetDC.Call(c.hwnd)
	if dc == 0 {
		return windowFrame{}, false
	}
	defer procReleaseDC.Call(c.hwnd, dc)
	mem, _, _ := procCreateCompatibleDC.Call(dc)
	if mem == 0 {
		return windowFrame{}, false
	}
	defer procDeleteDC.Call(mem)
	// A negative height makes the bitmap top-down, so the rows come out in
	// the order NDI wants them.
	info := bitmapInfo{header: bitmapInfoHeader{
		size:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		width:    int32(w),
		height:   -int32(h),
		planes:   1,
		bitCount: 32,
	}}
	var bits unsafe.Pointer
	bmp, _, _ := procCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&info)), dibRGBColors,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == nil {
		return windowFrame{}, false
	}
	defer procDeleteObject.Call(bmp)
	old, _, _ := procSelectObject.Call(mem, bmp)
	defer procSelectObject.Call(mem, old)

	if ok, _, _ := procPrintWindow.Call(c.hwnd, mem, printWindowFullContent); ok == 0 {
		if ok, _, _ = procBitBlt.Call(mem, 0, 0, uintptr(w), uintptr(h), dc, 0, 0, srcCopy); ok == 0 {
			return windowFrame{}, false
		}
	}

	stride := w * 4
	size := stride * h
	if cap(c.frame.pix) < size {
		c.frame.pix = make([]byte, size)
	}
	pix := c.frame.pix[:size]
	copy(pix, unsafe.Slice((*byte)(bits), size))
	// The bitmap leaves the fourth byte zero, which NDI reads as a fully
	// transparent pixel.
	for i := 3; i < size; i += 4 {
		pix[i] = 0xff
	}
	c.frame.stride, c.frame.w, c.frame.h = stride, w, h
	return c.frame, true
}
