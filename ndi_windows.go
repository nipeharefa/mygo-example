//go:build windows

package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/egoist/mygo"
)

// The NDI runtime publishes the same entry points on every platform, but
// purego cannot load a library on Windows, so this file binds them with
// syscall and calls through SyscallN.
const (
	ndiSourceName = "mygo-demo (Kubernetes)"
	ndiFrameRate  = 30

	ndiRuntimeDirV6 = "NDI_RUNTIME_DIR_V6"
	ndiRuntimeDirV5 = "NDI_RUNTIME_DIR_V5"

	printWindowFullContent = 2
	srcCopy                = 0x00CC0020
	dibRGBColors           = 0
	ndiFrameProgressive    = 1
)

// ndiSendCreate mirrors NDIlib_send_create_t: two strings, then two bools.
type ndiSendCreate struct {
	name       *byte
	groups     *byte
	clockVideo uint8
	clockAudio uint8
}

// ndiVideoFrame mirrors NDIlib_video_frame_v2_t, field for field. The
// padding Go inserts after frameFormatType is the same padding the C
// compiler inserts, which the layout test pins down.
type ndiVideoFrame struct {
	xres, yres             int32
	fourCC                 [4]byte
	frameRateN, frameRateD int32
	pictureAspectRatio     float32
	frameFormatType        int32
	timecode               int64
	data                   *byte
	lineStride             int32
	metadata               *byte
	timestamp              int64
}

// ndiLibrary holds the loaded runtime and the entry points used here.
type ndiLibrary struct {
	handle      uintptr
	initialize  uintptr
	sendCreate  uintptr
	sendVideo   uintptr
	sendDestroy uintptr
}

// ndiDLLName is the runtime the installer leaves behind, per architecture.
func ndiDLLName() string {
	if runtime.GOARCH == "arm64" {
		return "Processing.NDI.Lib.arm64.dll"
	}
	return "Processing.NDI.Lib.x64.dll"
}

// ndiDLLCandidates lists the places the installer may have left the
// runtime, most specific first. The bare name lets the loader search PATH.
// NDI Tools and the standalone NDI Runtime use different folder names, and
// either can be the only thing installed, so both are listed.
func ndiDLLCandidates() []string {
	name := ndiDLLName()
	paths := []string{name}
	for _, env := range []string{ndiRuntimeDirV6, ndiRuntimeDirV5} {
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
	return paths
}

// loadNDI finds the runtime and resolves what the sender needs. It tells a
// missing install apart from one that is there but cannot load, because
// the second sends an unhelpful "module not found" from Windows.
func loadNDI() (*ndiLibrary, error) {
	var broken error
	for _, path := range ndiDLLCandidates() {
		handle, err := syscall.LoadLibrary(path)
		if err != nil {
			if filepath.IsAbs(path) {
				if _, statErr := os.Stat(path); statErr == nil {
					broken = fmt.Errorf("the NDI runtime at %s could not be loaded: %w", path, err)
				}
			}
			continue
		}
		lib := &ndiLibrary{handle: uintptr(handle)}
		missing := false
		for _, sym := range []struct {
			name string
			addr *uintptr
		}{
			{"NDIlib_initialize", &lib.initialize},
			{"NDIlib_send_create_v2", &lib.sendCreate},
			{"NDIlib_send_send_video_v2", &lib.sendVideo},
			{"NDIlib_send_destroy", &lib.sendDestroy},
		} {
			addr, err := syscall.GetProcAddress(handle, sym.name)
			if err != nil {
				broken = fmt.Errorf("%s exports no %s: %w", path, sym.name, err)
				missing = true
				break
			}
			*sym.addr = addr
		}
		if missing {
			syscall.FreeLibrary(handle)
			continue
		}
		return lib, nil
	}
	if broken != nil {
		return nil, broken
	}
	return nil, fmt.Errorf("the NDI runtime is not installed, install NDI Tools: no %s on PATH, in %s, or under %s",
		ndiDLLName(), ndiRuntimeDirV6, filepath.Join(os.Getenv("ProgramFiles"), "NDI"))
}

// call invokes one of the entry points; Windows passes arguments in
// registers, so SyscallN is enough.
func (l *ndiLibrary) call(fn uintptr, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(fn, args...)
	return r
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

// windowFrame is one captured frame, in BGRA, as NDI expects it.
type windowFrame struct {
	pix    []byte
	stride int
	w, h   int
}

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
	// A negative height makes the bitmap top-down, so the rows come out
	// in the order NDI wants them.
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

// waitFrame retries capture until the window shows something, so a start
// fails with a message instead of a stream of nothing.
func (c *windowCapturer) waitFrame(timeout time.Duration) (windowFrame, bool) {
	deadline := time.Now().Add(timeout)
	for {
		if f, ok := c.capture(); ok {
			return f, true
		}
		if time.Now().After(deadline) {
			return windowFrame{}, false
		}
		time.Sleep(time.Second / ndiFrameRate)
	}
}

// ndiStream is a running sender: one goroutine captures and sends until
// Stop closes it.
type ndiStream struct {
	lib  *ndiLibrary
	send uintptr
	capt *windowCapturer
	stop chan struct{}
	done chan struct{}
	once sync.Once
}

// ndiSupported reports whether this build can publish the window.
func ndiSupported() bool { return true }

// startNDISender captures the window this process owns and publishes it
// as an NDI source. It must run on the main thread, because it reads the
// window handle.
func startNDISender(win *mygo.Window) (*ndiStream, error) {
	if win == nil {
		return nil, errors.New("no window to publish yet")
	}
	capt, err := newWindowCapturer(win.NativeHandle())
	if err != nil {
		return nil, err
	}
	if _, ok := capt.waitFrame(2 * time.Second); !ok {
		return nil, errors.New("cannot read the window's pixels")
	}
	lib, err := loadNDI()
	if err != nil {
		return nil, err
	}
	if ok := lib.call(lib.initialize); ok == 0 {
		return nil, errors.New("the NDI runtime failed to initialize")
	}
	name := append([]byte(ndiSourceName), 0)
	settings := ndiSendCreate{name: &name[0], clockVideo: 1}
	send := lib.call(lib.sendCreate, uintptr(unsafe.Pointer(&settings)))
	runtime.KeepAlive(name)
	if send == 0 {
		return nil, errors.New("the NDI runtime refused to create a sender")
	}
	s := &ndiStream{lib: lib, send: send, capt: capt, stop: make(chan struct{}), done: make(chan struct{})}
	go s.run()
	return s, nil
}

// run sends frames until Stop. Sending with the video clock on blocks
// until the frame is due, which paces the loop at the frame rate.
func (s *ndiStream) run() {
	defer close(s.done)
	frame := ndiVideoFrame{
		fourCC:          [4]byte{'B', 'G', 'R', 'A'},
		frameRateN:      ndiFrameRate,
		frameRateD:      1,
		frameFormatType: ndiFrameProgressive,
		timecode:        math.MaxInt64,
	}
	for {
		select {
		case <-s.stop:
			return
		default:
		}
		f, ok := s.capt.capture()
		if !ok {
			time.Sleep(time.Second / ndiFrameRate)
			continue
		}
		frame.xres, frame.yres = int32(f.w), int32(f.h)
		frame.pictureAspectRatio = float32(f.w) / float32(f.h)
		frame.data = &f.pix[0]
		frame.lineStride = int32(f.stride)
		s.lib.call(s.lib.sendVideo, s.send, uintptr(unsafe.Pointer(&frame)))
	}
}

// Stop ends the stream and unregisters the source.
func (s *ndiStream) Stop() {
	s.once.Do(func() {
		close(s.stop)
		<-s.done
		s.lib.call(s.lib.sendDestroy, s.send)
	})
}
