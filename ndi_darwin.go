//go:build darwin

package main

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

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

// ndiCaptureFailure explains a window that gave no pixels.
func ndiCaptureFailure() string {
	return "cannot read the window's pixels while it is off screen; bring the window into view to start the stream"
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

// captureFrame reads one image of the window; CoreGraphics is safe to call
// from the capture goroutine.
func captureFrame(c ndiCapturer) (windowFrame, bool) {
	return c.capture()
}

const (
	cgFrameworkPath = "/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics"
	objcFramework   = "/usr/lib/libobjc.A.dylib"

	// Flags and bitmap settings from CoreGraphics' headers.
	cgWindowListOptionIncludingWindow = 1 << 3 // kCGWindowListOptionIncludingWindow
	cgWindowImageBoundsIgnoreFraming  = 1 << 0 // kCGWindowImageBoundsIgnoreFraming
	cgWindowImageNominalResolution    = 1 << 4 // kCGWindowImageNominalResolution
	cgImageAlphaPremultipliedFirst    = 2      // kCGImageAlphaPremultipliedFirst
	cgBitmapByteOrder32Little         = 2 << 12
	cgInterpolationNone               = 1
)

// cgPoint, cgSize and cgRect mirror CGPoint, CGSize and CGRect: four
// float64 values laid out origin-then-size.
type (
	cgPoint struct{ X, Y float64 }
	cgSize  struct{ W, H float64 }
	cgRect  struct {
		Origin cgPoint
		Size   cgSize
	}
)

var (
	cgOnce sync.Once
	cgErr  error
	cgLib  uintptr
	cgSP   uintptr // the color space frames are drawn in

	cgWindowListCreateImage          func(bounds cgRect, list, window, options uint32) uintptr
	cgImageGetWidth                  func(img uintptr) uintptr
	cgImageGetHeight                 func(img uintptr) uintptr
	cgImageRelease                   func(img uintptr)
	cgBitmapContextCreate            func(data unsafe.Pointer, width, height, bitsPerComponent, bytesPerRow, space uintptr, info uint32) uintptr
	cgContextSetInterpolationQuality func(ctx uintptr, quality int32)
	cgContextDrawImage               func(ctx uintptr, rect cgRect, img uintptr)
	cgContextRelease                 func(ctx uintptr)
	cgColorSpaceCreateDeviceRGB      func() uintptr
	cgColorSpaceCreateWithName       func(name uintptr) uintptr

	objcOnce            sync.Once
	objcErr             error
	objcMsgSend         func(obj, sel uintptr) uintptr
	objcSelRegisterName func(name string) uintptr
)

// registerFunc resolves one symbol and binds it to fn.
func registerFunc(lib uintptr, fn any, name string) error {
	sym, err := purego.Dlsym(lib, name)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	purego.RegisterFunc(fn, sym)
	return nil
}

// loadCapture binds the CoreGraphics calls the capture loop uses.
func loadCapture() error {
	cgOnce.Do(func() {
		cgLib, cgErr = purego.Dlopen(cgFrameworkPath, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if cgErr != nil {
			return
		}
		for _, sym := range []struct {
			fn   any
			name string
		}{
			{&cgImageGetWidth, "CGImageGetWidth"},
			{&cgImageGetHeight, "CGImageGetHeight"},
			{&cgImageRelease, "CGImageRelease"},
			{&cgBitmapContextCreate, "CGBitmapContextCreate"},
			{&cgContextSetInterpolationQuality, "CGContextSetInterpolationQuality"},
			{&cgContextDrawImage, "CGContextDrawImage"},
			{&cgContextRelease, "CGContextRelease"},
			{&cgColorSpaceCreateDeviceRGB, "CGColorSpaceCreateDeviceRGB"},
			{&cgColorSpaceCreateWithName, "CGColorSpaceCreateWithName"},
		} {
			if err := registerFunc(cgLib, sym.fn, sym.name); err != nil {
				cgErr = err
				return
			}
		}
		// CGWindowListCreateImage is deprecated since macOS 14 and may be
		// gone from a later system; that is a clear error, not a broken
		// image.
		if env, err := purego.Dlsym(cgLib, "CGWindowListCreateImage"); err == nil {
			purego.RegisterFunc(&cgWindowListCreateImage, env)
		}
		cgSP = sRGBColorSpace()
	})
	return cgErr
}

// sRGBColorSpace returns the color space the window server renders in, so
// the frame NDI sends keeps its colors; the device space is the fallback.
func sRGBColorSpace() uintptr {
	if sym, err := purego.Dlsym(cgLib, "kCGColorSpaceSRGB"); err == nil {
		// The symbol holds a CFStringRef: read it as CoreGraphics' global.
		if name := **(**uintptr)(unsafe.Pointer(&sym)); name != 0 {
			if space := cgColorSpaceCreateWithName(name); space != 0 {
				return space
			}
		}
	}
	return cgColorSpaceCreateDeviceRGB()
}

// loadObjC binds objc_msgSend, used to read the window's number once.
func loadObjC() error {
	objcOnce.Do(func() {
		lib, err := purego.Dlopen(objcFramework, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			objcErr = fmt.Errorf("Objective-C runtime: %w", err)
			return
		}
		if err := registerFunc(lib, &objcMsgSend, "objc_msgSend"); err != nil {
			objcErr = err
			return
		}
		objcErr = registerFunc(lib, &objcSelRegisterName, "sel_registerName")
	})
	return objcErr
}

// windowCapturer grabs one window's image on a background goroutine.
type windowCapturer struct {
	number uint32
	frame  windowFrame
}

// newWindowCapturer resolves handle to the window number captures need.
// It must run on the main thread: AppKit answers there, and both callers
// (the header button and the launch hook) do.
func newWindowCapturer(handle uintptr) (*windowCapturer, error) {
	if err := loadCapture(); err != nil {
		return nil, err
	}
	if cgWindowListCreateImage == nil {
		return nil, errors.New("this macOS no longer offers window capture (CGWindowListCreateImage)")
	}
	if err := loadObjC(); err != nil {
		return nil, err
	}
	number := objcMsgSend(handle, objcSelRegisterName("windowNumber"))
	if number == 0 {
		return nil, errors.New("the window has no number yet")
	}
	return &windowCapturer{number: uint32(number)}, nil
}

// capture renders the window as the window server shows it: an app reads
// its own windows without the permission to record the screen. It reports
// false while the window has no image, which is what an off-screen window
// (minimized, hidden, behind a full screen app, or in another Space) gives.
func (c *windowCapturer) capture() (windowFrame, bool) {
	// CGRectNull asks for the window's own bounds, shadow excluded.
	null := cgRect{Origin: cgPoint{X: math.Inf(1), Y: math.Inf(1)}}
	img := cgWindowListCreateImage(null, cgWindowListOptionIncludingWindow, c.number,
		cgWindowImageBoundsIgnoreFraming|cgWindowImageNominalResolution)
	if img == 0 {
		return windowFrame{}, false
	}
	defer cgImageRelease(img)
	w, h := int(cgImageGetWidth(img)), int(cgImageGetHeight(img))
	if w <= 0 || h <= 0 {
		return windowFrame{}, false
	}
	if c.frame.w != w || c.frame.h != h {
		c.frame = windowFrame{w: w, h: h, stride: w * 4, pix: make([]byte, w*h*4)}
	}
	// Little-endian 32-bit with alpha first stores each pixel as B, G, R, A:
	// NDI's BGRA layout, with no per-pixel work to convert it.
	ctx := cgBitmapContextCreate(unsafe.Pointer(&c.frame.pix[0]), uintptr(w), uintptr(h),
		8, uintptr(c.frame.stride), cgSP, cgImageAlphaPremultipliedFirst|cgBitmapByteOrder32Little)
	if ctx == 0 {
		return windowFrame{}, false
	}
	cgContextSetInterpolationQuality(ctx, cgInterpolationNone)
	cgContextDrawImage(ctx, cgRect{Size: cgSize{W: float64(w), H: float64(h)}}, img)
	cgContextRelease(ctx)
	return c.frame, true
}
