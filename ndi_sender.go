//go:build darwin || linux || windows

package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
)

// The NDI runtime at runtime, shared by every platform that publishes the
// window. NDI 5 and 6 hand out their functions as one versioned table; the
// per-platform files only say where the library is and how to capture the
// window.
const (
	ndiSourceName = "mygo-demo (Live)"

	// ndiFrameRate paces the capture loop: sending with the video clock on
	// blocks until the next frame is due, so the loop cannot outrun it.
	ndiFrameRate = 30

	ndiFrameProgressive = 1

	// The environment variables each installer uses to say where its
	// runtime lives.
	ndiRuntimeDirV6  = "NDI_RUNTIME_DIR_V6"
	ndiRuntimeDirV5  = "NDI_RUNTIME_DIR_V5"
	ndiRuntimeDirAny = "NDI_RUNTIME_DIR"

	// The table only ever grows, so an entry keeps its index across
	// releases. These four come from Processing.NDI.DynamicLoad.h.
	ndiTableInitialize  = 0
	ndiTableSendCreate  = 8
	ndiTableSendDestroy = 9
	ndiTableSendVideo   = 50
	ndiTableSize        = ndiTableSendVideo + 1
)

// ndiLoadNames are the table entry points, newest first. A runtime exports
// the one it was built as, and the older names stay for compatibility.
var ndiLoadNames = []string{"NDIlib_v6_3_load", "NDIlib_v6_load", "NDIlib_v5_load"}

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

// ndiRuntime is one loaded library and the entry points the sender uses.
type ndiRuntime struct {
	close       func()
	initialize  func() bool
	sendCreate  func(*ndiSendCreate) uintptr
	sendDestroy func(uintptr)
	sendVideo   func(uintptr, *ndiVideoFrame)
}

// loadNDI opens the first runtime that offers a sender. A separate error
// for "found but will not load" matters, because Windows otherwise reports
// a bare "module not found" for a runtime that is installed.
func loadNDI() (*ndiRuntime, error) {
	var broken error
	for _, path := range ndiLibraryCandidates() {
		handle, err := openNDILibrary(path)
		if err != nil {
			if _, statErr := os.Stat(path); statErr == nil {
				broken = fmt.Errorf("the NDI runtime at %s could not be loaded: %w", path, err)
			}
			continue
		}
		rt, err := bindNDI(handle)
		if err != nil {
			closeNDILibrary(handle)
			broken = fmt.Errorf("%s: %w", path, err)
			continue
		}
		rt.close = func() { closeNDILibrary(handle) }
		return rt, nil
	}
	if broken != nil {
		return nil, broken
	}
	return nil, fmt.Errorf("the NDI runtime is not installed, install NDI Tools: no %s", ndiInstallHint())
}

// bindNDI resolves the entry points, through the versioned table first and
// the named exports second, which is what an older runtime offers.
func bindNDI(handle uintptr) (*ndiRuntime, error) {
	tableErr := errors.New("no versioned table")
	if rt, err := bindTable(handle); err == nil {
		return rt, nil
	} else {
		tableErr = err
	}
	if rt, err := bindNames(handle); err == nil {
		return rt, nil
	} else {
		return nil, fmt.Errorf("%w; and it exports no sender by name either: %w", tableErr, err)
	}
}

// bindTable calls the versioned load function and reads the pointers it
// returns. This is how NDI 5 and 6 expose the API on most builds.
func bindTable(handle uintptr) (*ndiRuntime, error) {
	var last error
	for _, name := range ndiLoadNames {
		addr, err := ndiSymbol(handle, name)
		if err != nil {
			last = fmt.Errorf("it exports no %s", name)
			continue
		}
		var load func() unsafe.Pointer
		purego.RegisterFunc(&load, addr)
		table := load()
		if table == nil {
			last = fmt.Errorf("%s returned no table", name)
			continue
		}
		entries := (*[ndiTableSize]uintptr)(table)
		if entries[ndiTableInitialize] == 0 || entries[ndiTableSendCreate] == 0 ||
			entries[ndiTableSendDestroy] == 0 || entries[ndiTableSendVideo] == 0 {
			last = fmt.Errorf("%s has gaps in the sender entries", name)
			continue
		}
		rt := &ndiRuntime{}
		purego.RegisterFunc(&rt.initialize, entries[ndiTableInitialize])
		purego.RegisterFunc(&rt.sendCreate, entries[ndiTableSendCreate])
		purego.RegisterFunc(&rt.sendDestroy, entries[ndiTableSendDestroy])
		purego.RegisterFunc(&rt.sendVideo, entries[ndiTableSendVideo])
		return rt, nil
	}
	return nil, last
}

// bindNames binds the functions directly, which is how an older runtime
// exports them.
func bindNames(handle uintptr) (*ndiRuntime, error) {
	rt := &ndiRuntime{}
	for _, sym := range []struct {
		name string
		fn   any
	}{
		{"NDIlib_initialize", &rt.initialize},
		{"NDIlib_send_create", &rt.sendCreate},
		{"NDIlib_send_destroy", &rt.sendDestroy},
		{"NDIlib_send_send_video_v2", &rt.sendVideo},
	} {
		addr, err := ndiSymbol(handle, sym.name)
		if err != nil {
			return nil, fmt.Errorf("it exports no %s", sym.name)
		}
		purego.RegisterFunc(sym.fn, addr)
	}
	return rt, nil
}

// windowFrame is one captured image, top row first, four BGRA bytes a
// pixel.
type windowFrame struct {
	pix    []byte
	stride int
	w, h   int
}

// ndiCapturer grabs one image of the app's own window.
type ndiCapturer interface {
	capture() (windowFrame, bool)
}

// waitFrame retries capture until the window shows something, so a start
// fails with a message instead of a stream of nothing.
func waitFrame(c ndiCapturer, timeout time.Duration) (windowFrame, bool) {
	deadline := time.Now().Add(timeout)
	for {
		if f, ok := captureFrame(c); ok {
			return f, true
		}
		if time.Now().After(deadline) {
			return windowFrame{}, false
		}
		time.Sleep(time.Second / ndiFrameRate)
	}
}

// ndiStream publishes a window as one NDI source until Stop.
type ndiStream struct {
	rt   *ndiRuntime
	send uintptr
	capt ndiCapturer
	stop chan struct{}
	done chan struct{}
	once sync.Once
}

// startNDISender opens the NDI runtime and starts sending frames of win. It
// must run on the main thread, because it reads the window handle.
func startNDISender(win *mygo.Window) (*ndiStream, error) {
	if win == nil {
		return nil, errors.New("NDI output needs the live window")
	}
	capt, err := newWindowCapturer(win.NativeHandle())
	if err != nil {
		return nil, err
	}
	if _, ok := waitFrame(capt, 2*time.Second); !ok {
		return nil, errors.New(ndiCaptureFailure())
	}
	rt, err := loadNDI()
	if err != nil {
		return nil, err
	}
	ok := rt.initialize()
	if !ok {
		rt.close()
		return nil, errors.New("the NDI runtime failed to initialize")
	}
	name := append([]byte(ndiSourceName), 0)
	settings := ndiSendCreate{name: &name[0], clockVideo: 1}
	send := rt.sendCreate(&settings)
	runtime.KeepAlive(name)
	if send == 0 {
		rt.close()
		return nil, errors.New("the NDI runtime refused to create a sender")
	}
	s := &ndiStream{rt: rt, send: send, capt: capt, stop: make(chan struct{}), done: make(chan struct{})}
	go s.run()
	return s, nil
}

// run captures and sends until Stop closes the stop channel.
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
		shot, ok := captureFrame(s.capt)
		if !ok {
			// Nothing to show right now (the window is minimized, hidden,
			// or on a screen reader that refuses); the consumer keeps the
			// last frame.
			select {
			case <-s.stop:
				return
			case <-time.After(time.Second / ndiFrameRate):
			}
			continue
		}
		frame.xres, frame.yres = int32(shot.w), int32(shot.h)
		frame.pictureAspectRatio = float32(shot.w) / float32(shot.h)
		frame.data = &shot.pix[0]
		frame.lineStride = int32(shot.stride)
		s.rt.sendVideo(s.send, &frame)
	}
}

// Stop stops the capture loop and removes the source from discovery.
func (s *ndiStream) Stop() {
	s.once.Do(func() {
		close(s.stop)
		<-s.done
		s.rt.sendDestroy(s.send)
		s.rt.close()
	})
}

// ndiSupported reports whether this build can publish the window.
func ndiSupported() bool { return true }
