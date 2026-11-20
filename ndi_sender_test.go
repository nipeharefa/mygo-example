//go:build darwin || linux || windows

package main

import (
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
)

// The versioned table is what makes one binary work against NDI 5 and NDI 6:
// every version hands back the same append-only array of function pointers
// and the indices below never move. A runtime that is present on this
// machine proves the indices against its own named exports; CI machines have
// no runtime, so they skip.
func TestNDITableMatchesNamedExports(t *testing.T) {
	var (
		handle uintptr
		err    error
	)
	for _, path := range ndiLibraryCandidates() {
		if handle, err = openNDILibrary(path); err == nil {
			break
		}
	}
	if handle == 0 {
		t.Skipf("no NDI runtime to compare against: %v", err)
	}
	defer closeNDILibrary(handle)

	addr, err := ndiSymbol(handle, "NDIlib_v5_load")
	if err != nil {
		t.Skipf("this runtime has no NDIlib_v5_load: %v", err)
	}
	var load func() unsafe.Pointer
	purego.RegisterFunc(&load, addr)
	table := load()
	if table == nil {
		t.Fatal("NDIlib_v5_load handed back no table")
	}
	entries := (*[ndiTableSize]uintptr)(table)

	for index, name := range map[int]string{
		ndiTableInitialize:  "NDIlib_initialize",
		ndiTableSendCreate:  "NDIlib_send_create",
		ndiTableSendDestroy: "NDIlib_send_destroy",
		ndiTableSendVideo:   "NDIlib_send_send_video_v2",
	} {
		named, err := ndiSymbol(handle, name)
		if err != nil {
			t.Fatalf("this runtime exports no %s: %v", name, err)
		}
		if entries[index] != named {
			t.Errorf("table index %d is not %s, so the indices in ndi_sender.go are wrong", index, name)
		}
	}
}
