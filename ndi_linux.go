//go:build linux

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
)

const (
	gtkLibrary     = "libgtk-3.so.0"
	gdkLibrary     = "libgdk-3.so.0"
	gobjectLibrary = "libgobject-2.0.so.0"
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

// ndiCaptureFailure explains a window that gave no pixels.
func ndiCaptureFailure() string {
	return "cannot read the window's pixels, which Wayland does not allow"
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

// captureFrame reads one image of the window on the main thread, because
// GDK is not thread safe and the capture loop runs on its own goroutine.
// RunOnMain runs the function straight away when it is already on the main
// thread, so this also works from the start path.
func captureFrame(c ndiCapturer) (frame windowFrame, ok bool) {
	mygo.RunOnMain(func() { frame, ok = c.capture() })
	return frame, ok
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
