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
)

// The NDI runtime at runtime, shared by every platform that publishes the
// stage. NDI 5 and 6 hand out their functions as one versioned table; the
// per-platform files only say where the library is and how to reach it.
const (

	// ndiFrameRate paces the frame loop: sending with the video clock on
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

	// The NDI feed is capped at HD. The stage renders at whatever size it
	// is asked for, and a larger one costs bandwidth for no visible gain
	// on a projector.
	ndiMaxFrameWidth  = 1920
	ndiMaxFrameHeight = 1080
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
// exports them. It calls purego.RegisterFunc itself: there is no
// per-platform registerFunc helper.
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

// ndiFrame is one image for the feed, top row first, four BGRA bytes a
// pixel.
type ndiFrame struct {
	pix    []byte
	stride int
	w, h   int
}

// ndiFrameSource draws one frame for the feed at the requested size.
type ndiFrameSource interface {
	frame(w, h int) (ndiFrame, bool)
}

// ndiStream publishes one source's frames as one NDI source until Stop.
type ndiStream struct {
	rt   *ndiRuntime
	send uintptr
	src  ndiFrameSource
	w, h int
	stop chan struct{}
	done chan struct{}
	once sync.Once

	mu        sync.Mutex
	frames    int64 // frames handed to the sender
	dropped   int64 // draws that had nothing to send
	width     int   // the last frame's size, which is what the feed sends
	height    int
	first     time.Time // when the first frame went out; zero until then
	lastFrame time.Time // when the last frame went out
	lastErr   string
}

// ndiStats is what the Settings page reports about the stream. NDI 5 and 6
// have no receiver tally, so these are the sender's own numbers.
type ndiStats struct {
	Frames, Dropped int64
	Width, Height   int
	FPS             float64 // measured since the first frame
	Uptime          time.Duration
	Err             string
}

// snapshot reads the counters for a view. The frame rate is measured, not
// taken from the frame header, so a stalled source shows a falling rate.
func (s *ndiStream) snapshot() ndiStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := ndiStats{
		Frames:  s.frames,
		Dropped: s.dropped,
		Width:   s.width,
		Height:  s.height,
		Uptime:  time.Since(s.first),
		Err:     s.lastErr,
	}
	if s.frames > 0 {
		if secs := s.lastFrame.Sub(s.first).Seconds(); secs >= time.Second.Seconds() {
			st.FPS = float64(s.frames-1) / secs
		}
	}
	return st
}

// sent records one frame on its way to the consumer.
func (s *ndiStream) sent(w, h int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if s.first.IsZero() {
		s.first = now
	}
	s.lastFrame = now
	s.frames++
	s.width, s.height = w, h
}

// skipped records a draw that found nothing to send.
func (s *ndiStream) skipped() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropped++
}

// failed records the last error the loop ran into.
func (s *ndiStream) failed(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastErr = err.Error()
}

// ndiRuntimeOnce initializes the loaded NDI library once for the process.
var (
	ndiRuntimeOnce sync.Once
	ndiRuntimeErr  error
)

// startNDISender opens the NDI runtime and starts sending frames of the
// source at w×h. newSource builds that source from the stream's stop
// channel, so it is called once the stream exists and can give up when the
// stream stops.
func startNDISender(newSource func(stop chan struct{}) ndiFrameSource, w, h int) (*ndiStream, error) {
	rt, err := loadNDI()
	if err != nil {
		return nil, err
	}
	// The library stays loaded and initialized for the process, which is
	// what Processing.NDI.Lib.h asks a dynamic-loading host to do; Stop
	// removes the source, not the library.
	ndiRuntimeOnce.Do(func() {
		if !rt.initialize() {
			ndiRuntimeErr = errors.New("the NDI runtime failed to initialize")
		}
	})
	if ndiRuntimeErr != nil {
		return nil, ndiRuntimeErr
	}
	name := append([]byte(stageSourceName), 0)
	settings := ndiSendCreate{name: &name[0], clockVideo: 1}
	send := rt.sendCreate(&settings)
	runtime.KeepAlive(name)
	if send == 0 {
		// The library stays loaded: it was initialized for the process.
		return nil, errors.New("the NDI runtime refused to create a sender")
	}
	s := &ndiStream{rt: rt, send: send, w: w, h: h, stop: make(chan struct{}), done: make(chan struct{})}
	s.src = newSource(s.stop)
	go s.run()
	// The sender exists by now, so a source that cannot draw is a clear
	// error instead of source discovery for a stream of nothing.
	if _, ok := s.src.frame(w, h); !ok {
		s.Stop()
		return nil, errors.New(ndiCaptureFailure())
	}
	return s, nil
}

// run draws and sends until Stop closes the stop channel.
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
		shot, ok := s.src.frame(s.w, s.h)
		if !ok {
			s.skipped()
			// Nothing to show right now (the stage is not rendering yet,
			// or the main thread is busy); the consumer keeps the last
			// frame.
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
		s.sent(int(frame.xres), int(frame.yres))
	}
}

// Stop stops the frame loop and removes the source from discovery.
func (s *ndiStream) Stop() {
	s.once.Do(func() {
		close(s.stop)
		<-s.done
		s.rt.sendDestroy(s.send)
		s.rt.close()
	})
}

// ndiSupported reports whether this build can publish the stage.
func ndiSupported() bool { return true }
