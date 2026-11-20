//go:build linux

package main

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/bitfocus/gondi"
	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
)

const (
	ndiSourceName = "mygo-demo (Kubernetes)"
	ndiFrameRate  = 30

	gtkLibrary     = "libgtk-3.so.0"
	gdkLibrary     = "libgdk-3.so.0"
	gobjectLibrary = "libgobject-2.0.so.0"
)

// ndiLibraryPaths is where the NDI Linux runtime usually lands; the bare
// name is tried first so an ld path setup still wins.
var ndiLibraryPaths = []string{
	"libndi.so",
	"/usr/local/lib/libndi.so",
	"/usr/lib/libndi.so",
}

// gdkPixbuf is the bookkeeping for reading a window's pixels through GDK.
// Going through GdkPixbuf keeps the X11 and Wayland case in one API and
// avoids mirroring XImage's layout by hand.
type gdkPixbuf struct {
	widgetWindow func(widget uintptr) uintptr
	fromWindow   func(window uintptr, x, y, w, h int32) uintptr
	getWidth     func(pixbuf uintptr) int32
	getHeight    func(pixbuf uintptr) int32
	getStride    func(pixbuf uintptr) int32
	getChans     func(pixbuf uintptr) int32
	hasAlpha     func(pixbuf uintptr) int32
	getPixels    func(pixbuf uintptr) unsafe.Pointer
	windowW      func(window uintptr) int32
	windowH      func(window uintptr) int32
	unref        func(obj uintptr)
}

// loadGDK binds the handful of GTK, GDK and GObject entry points used
// here. RegisterFunc panics on a symbol mismatch, so the panic becomes an
// error instead.
func loadGDK() (g *gdkPixbuf, err error) {
	defer func() {
		if r := recover(); r != nil {
			g, err = nil, fmt.Errorf("the GTK runtime is unusable: %v", r)
		}
	}()
	gtk, err := purego.Dlopen(gtkLibrary, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, fmt.Errorf("no GTK runtime: %w", err)
	}
	gdk, err := purego.Dlopen(gdkLibrary, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, fmt.Errorf("no GDK runtime: %w", err)
	}
	gobj, err := purego.Dlopen(gobjectLibrary, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, fmt.Errorf("no GObject runtime: %w", err)
	}
	g = &gdkPixbuf{}
	for _, b := range []struct {
		lib  uintptr
		name string
		addr any
	}{
		{gtk, "gtk_widget_get_window", &g.widgetWindow},
		{gdk, "gdk_pixbuf_get_from_window", &g.fromWindow},
		{gdk, "gdk_pixbuf_get_width", &g.getWidth},
		{gdk, "gdk_pixbuf_get_height", &g.getHeight},
		{gdk, "gdk_pixbuf_get_rowstride", &g.getStride},
		{gdk, "gdk_pixbuf_get_n_channels", &g.getChans},
		{gdk, "gdk_pixbuf_get_has_alpha", &g.hasAlpha},
		{gdk, "gdk_pixbuf_get_pixels", &g.getPixels},
		{gdk, "gdk_window_get_width", &g.windowW},
		{gdk, "gdk_window_get_height", &g.windowH},
		{gobj, "g_object_unref", &g.unref},
	} {
		addr, err := purego.Dlsym(b.lib, b.name)
		if err != nil {
			return nil, fmt.Errorf("the GTK runtime exports no %s: %w", b.name, err)
		}
		purego.RegisterFunc(b.addr, addr)
	}
	return g, nil
}

// windowFrame is one captured frame, in BGRA, as NDI expects it.
type windowFrame struct {
	pix    []byte
	stride int
	w, h   int
}

// windowCapturer copies the app's own window out of GDK. Every GDK call
// must happen on the main thread, so capture is only ever run through
// mygo.RunOnMain.
type windowCapturer struct {
	gdk    *gdkPixbuf
	widget uintptr
	frame  windowFrame
}

// newWindowCapturer binds GTK and remembers the window. It must run on the
// main thread, like every other GDK call here.
func newWindowCapturer(handle uintptr) (*windowCapturer, error) {
	if handle == 0 {
		return nil, errors.New("the window has no handle yet")
	}
	gdk, err := loadGDK()
	if err != nil {
		return nil, err
	}
	return &windowCapturer{gdk: gdk, widget: handle}, nil
}

// capture asks GDK for the window's pixels and rewrites them as BGRA.
// Main thread only: GDK is not thread safe.
func (c *windowCapturer) capture() (windowFrame, bool) {
	window := c.gdk.widgetWindow(c.widget)
	if window == 0 {
		return windowFrame{}, false
	}
	w := int(c.gdk.windowW(window))
	h := int(c.gdk.windowH(window))
	if w <= 0 || h <= 0 {
		return windowFrame{}, false
	}
	pixbuf := c.gdk.fromWindow(window, 0, 0, int32(w), int32(h))
	if pixbuf == 0 {
		// Wayland hands out no readback of a window's pixels; X11 does.
		return windowFrame{}, false
	}
	defer c.gdk.unref(pixbuf)

	pw, ph := int(c.gdk.getWidth(pixbuf)), int(c.gdk.getHeight(pixbuf))
	stride := int(c.gdk.getStride(pixbuf))
	channels := int(c.gdk.getChans(pixbuf))
	alpha := c.gdk.hasAlpha(pixbuf) != 0
	if pw <= 0 || ph <= 0 || stride <= 0 || channels < 3 {
		return windowFrame{}, false
	}
	src := unsafe.Slice((*byte)(c.gdk.getPixels(pixbuf)), stride*ph)

	outStride := pw * 4
	size := outStride * ph
	if cap(c.frame.pix) < size {
		c.frame.pix = make([]byte, size)
	}
	dst := c.frame.pix[:size]
	for y := 0; y < ph; y++ {
		row := src[y*stride:]
		out := dst[y*outStride:]
		for x := 0; x < pw; x++ {
			in := row[x*channels:]
			// GdkPixbuf stores RGBA; NDI wants BGRA.
			out[x*4+0] = in[2]
			out[x*4+1] = in[1]
			out[x*4+2] = in[0]
			if alpha {
				out[x*4+3] = in[3]
			} else {
				out[x*4+3] = 0xff
			}
		}
	}
	runtime.KeepAlive(pixbuf)
	c.frame.stride, c.frame.w, c.frame.h = outStride, pw, ph
	return c.frame, true
}

// waitFrame retries capture until the window shows something. Main thread
// only.
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
	send *gondi.SendInstance
	capt *windowCapturer
	stop chan struct{}
	done chan struct{}
	once sync.Once
}

// ndiSupported reports whether this build can publish the window.
func ndiSupported() bool { return true }

// initGondi loads the NDI runtime once and turns gondi's panics into an
// error, since a missing runtime is an ordinary case here.
var gondiOnce sync.Once

func initGondi() (err error) {
	gondiOnce.Do(func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("the NDI runtime is not installed, install NDI Tools: %v", r)
			}
		}()
		for _, path := range ndiLibraryPaths {
			if err = gondi.InitLibrary(path); err == nil {
				return
			}
		}
		err = fmt.Errorf("the NDI runtime is not installed, install NDI Tools: %v", err)
	})
	return err
}

// startNDISender captures the window this process owns and publishes it as
// an NDI source. It must run on the main thread.
func startNDISender(win *mygo.Window) (*ndiStream, error) {
	if win == nil {
		return nil, errors.New("no window to publish yet")
	}
	capt, err := newWindowCapturer(win.NativeHandle())
	if err != nil {
		return nil, err
	}
	if _, ok := capt.waitFrame(2 * time.Second); !ok {
		return nil, errors.New("cannot read the window's pixels, which Wayland does not allow")
	}
	if err := initGondi(); err != nil {
		return nil, err
	}
	send, err := gondi.NewSendInstance(ndiSourceName, "", true, false)
	if err != nil {
		return nil, fmt.Errorf("the NDI runtime refused to create a sender: %w", err)
	}
	s := &ndiStream{send: send, capt: capt, stop: make(chan struct{}), done: make(chan struct{})}
	go s.run()
	return s, nil
}

// run sends frames until Stop. Sending with the video clock on blocks
// until the frame is due, which paces the loop at the frame rate.
func (s *ndiStream) run() {
	defer close(s.done)
	frame := gondi.NewVideoFrameV2()
	frame.FourCC = gondi.FourCCTypeBGRA
	frame.FrameRateN, frame.FrameRateD = ndiFrameRate, 1
	frame.FrameFormatType = gondi.FrameFormatProgressive
	frame.Timecode = math.MaxInt64
	for {
		select {
		case <-s.stop:
			return
		default:
		}
		var f windowFrame
		var ok bool
		mygo.RunOnMain(func() { f, ok = s.capt.capture() })
		if !ok {
			time.Sleep(time.Second / ndiFrameRate)
			continue
		}
		frame.Xres, frame.Yres = int32(f.w), int32(f.h)
		frame.PictureAspectRatio = float32(f.w) / float32(f.h)
		frame.Data = &f.pix[0]
		frame.LineStride = int32(f.stride)
		s.send.SendVideoFrame(frame)
	}
}

// Stop ends the stream and unregisters the source.
func (s *ndiStream) Stop() {
	s.once.Do(func() {
		close(s.stop)
		<-s.done
		s.send.Destroy()
	})
}
