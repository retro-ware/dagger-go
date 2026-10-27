//go:build darwin

package gpu

import (
	"sync"
	"unsafe"

	"github.com/go-webgpu/goffi/ffi"
	"github.com/go-webgpu/goffi/types"
)

// The pointer on macOS, which gogpu can neither move nor lock (its locked mode is a stub
// there, and it drops the events of a pointer outside the window): CoreGraphics moves it,
// each tick reads where it is, it is hidden only while over the picture (NSCursor's hide
// is the whole screen's), and a lock freezes it hidden at the window's centre, the mouse's
// motion coming from its events (gogpu's pointer deltas): a frozen pointer can't reach the
// Dock or a hot corner.

// point is a CGPoint (NSPoint): x and y in screen points.
type point struct{ x, y float64 }

var mac struct {
	once sync.Once
	ok   bool

	msgSend, warp, associate, eventCreate, eventLocation, release unsafe.Pointer

	send      types.CallInterface // id objc_msgSend(id, SEL)
	sendPoint types.CallInterface // NSPoint objc_msgSend(id, SEL)
	ptr       types.CallInterface // void *f(void *)
	getPoint  types.CallInterface // CGPoint f(void *)
	warpCIF   types.CallInterface // CGError f(CGPoint)
	assocCIF  types.CallInterface // CGError f(boolean_t)

	app, cursor                              uintptr // NSApp, the NSCursor class
	keyWindow, mouseLocation, hideS, unhideS uintptr // selectors

	local  point // the pointer on the window's content, in points from its top left
	hide   bool  // SetCursorHidden(true): hidden over the picture
	hidden bool  // NSCursor's hide is in effect (it counts: hidden once at most)
	frozen bool  // the pointer is detached from the mouse
}

func macInit() bool {
	mac.once.Do(func() {
		lib := func(path string) unsafe.Pointer { h, _ := ffi.LoadLibrary(path); return h }
		objc := lib("/usr/lib/libobjc.A.dylib")
		cg := lib("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics")
		cf := lib("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
		if objc == nil || cg == nil || cf == nil {
			return
		}
		sym := func(h unsafe.Pointer, name string) unsafe.Pointer { p, _ := ffi.GetSymbol(h, name); return p }
		getClass, selName := sym(objc, "objc_getClass"), sym(objc, "sel_registerName")
		mac.msgSend = sym(objc, "objc_msgSend")
		mac.warp = sym(cg, "CGWarpMouseCursorPosition")
		mac.associate = sym(cg, "CGAssociateMouseAndMouseCursorPosition")
		mac.eventCreate = sym(cg, "CGEventCreate")
		mac.eventLocation = sym(cg, "CGEventGetLocation")
		mac.release = sym(cf, "CFRelease")
		for _, p := range []unsafe.Pointer{getClass, selName, mac.msgSend, mac.warp, mac.associate,
			mac.eventCreate, mac.eventLocation, mac.release} {
			if p == nil {
				return
			}
		}
		P, U := types.PointerTypeDescriptor, types.UInt32TypeDescriptor
		pt := &types.TypeDescriptor{Size: 16, Alignment: 8, Kind: types.StructType,
			Members: []*types.TypeDescriptor{types.DoubleTypeDescriptor, types.DoubleTypeDescriptor}}
		prep := func(cif *types.CallInterface, ret *types.TypeDescriptor, args ...*types.TypeDescriptor) bool {
			return ffi.PrepareCallInterface(cif, types.DefaultCall, ret, args) == nil
		}
		if !prep(&mac.send, P, P, P) || !prep(&mac.sendPoint, pt, P, P) || !prep(&mac.ptr, P, P) ||
			!prep(&mac.getPoint, pt, P) || !prep(&mac.warpCIF, U, pt) || !prep(&mac.assocCIF, U, U) {
			return
		}
		name := func(fn unsafe.Pointer, s string) uintptr {
			b := append([]byte(s), 0)
			p := unsafe.Pointer(&b[0])
			var r uintptr
			_, _ = ffi.CallFunction(&mac.ptr, fn, unsafe.Pointer(&r), []unsafe.Pointer{unsafe.Pointer(&p)})
			return r
		}
		mac.app = macSend(name(getClass, "NSApplication"), name(selName, "sharedApplication"))
		mac.cursor = name(getClass, "NSCursor")
		mac.keyWindow = name(selName, "keyWindow")
		mac.mouseLocation = name(selName, "mouseLocationOutsideOfEventStream")
		mac.hideS, mac.unhideS = name(selName, "hide"), name(selName, "unhide")
		mac.ok = mac.app != 0 && mac.cursor != 0
	})
	return mac.ok
}

func macSend(id, sel uintptr) uintptr {
	var r uintptr
	_, _ = ffi.CallFunction(&mac.send, mac.msgSend, unsafe.Pointer(&r), []unsafe.Pointer{unsafe.Pointer(&id), unsafe.Pointer(&sel)})
	return r
}

// macCall calls f(arg), arg a C pointer, for its pointer result.
func macCall(f unsafe.Pointer, arg uintptr) uintptr {
	var r uintptr
	_, _ = ffi.CallFunction(&mac.ptr, f, unsafe.Pointer(&r), []unsafe.Pointer{unsafe.Pointer(&arg)})
	return r
}

// macPointer is the pointer on the key window's content (in points from its top left) and
// on the screen; false without a key window (the game is in the background).
func macPointer() (local, screen point, ok bool) {
	w := macSend(mac.app, mac.keyWindow)
	if w == 0 {
		return
	}
	sel := mac.mouseLocation
	var p point // in the window's coordinates, from its bottom left
	_, _ = ffi.CallFunction(&mac.sendPoint, mac.msgSend, unsafe.Pointer(&p), []unsafe.Pointer{unsafe.Pointer(&w), unsafe.Pointer(&sel)})
	ev := macCall(mac.eventCreate, 0) // an event's location is the pointer's, now
	if ev == 0 {
		return
	}
	_, _ = ffi.CallFunction(&mac.getPoint, mac.eventLocation, unsafe.Pointer(&screen), []unsafe.Pointer{unsafe.Pointer(&ev)})
	macCall(mac.release, ev)
	_, h := win.app.Size()
	return point{p.x, float64(h) - p.y}, screen, true
}

// macWarp moves the pointer to p on the screen.
func macWarp(p point) {
	var r uint64
	_, _ = ffi.CallFunction(&mac.warpCIF, mac.warp, unsafe.Pointer(&r), []unsafe.Pointer{unsafe.Pointer(&p)})
	macFreeze(mac.frozen) // without it, the moved pointer stands still for a quarter of a second
}

// macFreeze detaches the pointer from the mouse (the mouse still sends its motion) or
// attaches it again.
func macFreeze(on bool) {
	mac.frozen = on
	var r uint32
	attach := uint32(1)
	if on {
		attach = 0
	}
	_, _ = ffi.CallFunction(&mac.assocCIF, mac.associate, unsafe.Pointer(&r), []unsafe.Pointer{unsafe.Pointer(&attach)})
}

// macShow shows or hides the pointer.
func macShow(on bool) {
	if mac.hidden == !on {
		return
	}
	mac.hidden = !on
	sel := mac.hideS
	if on {
		sel = mac.unhideS
	}
	macSend(mac.cursor, sel)
}

// macCentre is the window's centre on the screen, from where the pointer is on both.
func macCentre(local, screen point) point {
	w, h := win.app.Size()
	return point{screen.x + float64(w)/2 - local.x, screen.y + float64(h)/2 - local.y}
}

// SetCursorPosition moves the pointer to (x, y) on the screen image (CursorPosition's
// inverse): to the pixel's centre, so that CursorPosition reads it back.
func SetCursorPosition(x, y int) {
	if win.app == nil || win.k == 0 || !macInit() {
		return
	}
	local, screen, ok := macPointer()
	if !ok {
		return
	}
	s := Monitor().DeviceScaleFactor()
	to := point{((float64(x)+0.5)*win.k + win.ox) / s, ((float64(y)+0.5)*win.k + win.oy) / s}
	macWarp(point{screen.x + to.x - local.x, screen.y + to.y - local.y})
	mac.local = to
}

// cursorCapture freezes the pointer, hidden at the window's centre, or frees it where it is
// (the game then moves it to its cursor).
func cursorCapture(on bool) {
	if !macInit() {
		return
	}
	if !on {
		macFreeze(false)
		return
	}
	if local, screen, ok := macPointer(); ok {
		macWarp(macCentre(local, screen))
		macFreeze(true)
		macShow(false)
	}
}

// cursorHide hides the pointer while it is over the picture.
func cursorHide(on bool) { mac.hide = on }

// pollCursor reads the free pointer at each tick, hidden over the picture. Without a key
// window (the game in the background) the pointer is attached and shown.
func pollCursor() {
	if win.app == nil || !macInit() {
		return
	}
	local, screen, ok := macPointer()
	if !ok {
		if mac.frozen {
			macFreeze(false)
		}
		macShow(true)
		return
	}
	if capture.on {
		if !mac.frozen { // the window came back while still locked
			macWarp(macCentre(local, screen))
			macFreeze(true)
		}
		macShow(false)
		return
	}
	mac.local = local
	s := Monitor().DeviceScaleFactor()
	x, y := (local.x*s-win.ox)/win.k, (local.y*s-win.oy)/win.k
	over := win.k != 0 && x >= 0 && y >= 0 && x < float64(win.sw) && y < float64(win.sh)
	macShow(!mac.hide || !over)
}

// cursorWindowPos is the pointer on the window, in its units.
func cursorWindowPos() (float64, float64) {
	if !mac.ok {
		return in.cursor[0], in.cursor[1]
	}
	return mac.local.x, mac.local.y
}
