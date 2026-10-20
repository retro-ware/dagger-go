package host

import (
	"github.com/gogpu/gpucontext"

	"fmt"
	"os"
	"time"

	"gpu"
)

// scan is a key's PC set-1 scan code; 0xE0 in the high byte marks a grey (extended) key.
var scan = map[gpu.Key]uint16{
	gpu.KeyEscape: 0x01, gpu.Key1: 0x02, gpu.Key2: 0x03, gpu.Key3: 0x04, gpu.Key4: 0x05,
	gpu.Key5: 0x06, gpu.Key6: 0x07, gpu.Key7: 0x08, gpu.Key8: 0x09, gpu.Key9: 0x0A,
	gpu.Key0: 0x0B, gpu.KeyMinus: 0x0C, gpu.KeyEqual: 0x0D, gpu.KeyBackspace: 0x0E,
	gpu.KeyTab: 0x0F, gpu.KeyQ: 0x10, gpu.KeyW: 0x11, gpu.KeyE: 0x12, gpu.KeyR: 0x13,
	gpu.KeyT: 0x14, gpu.KeyY: 0x15, gpu.KeyU: 0x16, gpu.KeyI: 0x17, gpu.KeyO: 0x18,
	gpu.KeyP: 0x19, keyLeftBracket: 0x1A, keyRightBracket: 0x1B, gpu.KeyEnter: 0x1C,
	keyLeftControl: 0x1D, gpu.KeyA: 0x1E, gpu.KeyS: 0x1F, gpu.KeyD: 0x20, gpu.KeyF: 0x21,
	gpu.KeyG: 0x22, gpu.KeyH: 0x23, gpu.KeyJ: 0x24, gpu.KeyK: 0x25, gpu.KeyL: 0x26,
	keySemicolon: 0x27, keyApostrophe: 0x28, keyGrave: 0x29, keyLeftShift: 0x2A,
	keyBackslash: 0x2B, gpu.KeyZ: 0x2C, gpu.KeyX: 0x2D, gpu.KeyC: 0x2E, gpu.KeyV: 0x2F,
	gpu.KeyB: 0x30, gpu.KeyN: 0x31, gpu.KeyM: 0x32, gpu.KeyComma: 0x33, gpu.KeyPeriod: 0x34,
	gpu.KeySlash: 0x35, keyRightShift: 0x36, keyKPMultiply: 0x37, keyLeftAlt: 0x38,
	gpu.KeySpace: 0x39, keyCapsLock: 0x3A, gpu.KeyF1: 0x3B, gpu.KeyF2: 0x3C, gpu.KeyF3: 0x3D,
	gpu.KeyF4: 0x3E, gpu.KeyF5: 0x3F, gpu.KeyF6: 0x40, gpu.KeyF7: 0x41, gpu.KeyF8: 0x42,
	gpu.KeyF9: 0x43, gpu.KeyF10: 0x44, keyNumLock: 0x45, keyScrollLock: 0x46,
	gpu.KeyKP7: 0x47, gpu.KeyKP8: 0x48, gpu.KeyKP9: 0x49, keyKPSubtract: 0x4A,
	gpu.KeyKP4: 0x4B, gpu.KeyKP5: 0x4C, gpu.KeyKP6: 0x4D, keyKPAdd: 0x4E, gpu.KeyKP1: 0x4F,
	gpu.KeyKP2: 0x50, gpu.KeyKP3: 0x51, gpu.KeyKP0: 0x52, keyKPDecimal: 0x53,
	gpu.KeyF11: 0x57, gpu.KeyF12: 0x58,
	gpu.KeyKPEnter: 0xE01C, keyRightControl: 0xE01D, keyKPDivide: 0xE035,
	keyRightAlt: 0xE038, gpu.KeyHome: 0xE047, gpu.KeyUp: 0xE048, gpu.KeyPageUp: 0xE049,
	gpu.KeyLeft: 0xE04B, gpu.KeyRight: 0xE04D, gpu.KeyEnd: 0xE04F, gpu.KeyDown: 0xE050,
	gpu.KeyPageDown: 0xE051, gpu.KeyInsert: 0xE052, gpu.KeyDelete: 0xE053,
}

// gogpu's codes for the keys gpu has no names for.
const (
	keyApostrophe   gpu.Key = gpu.Key(gpucontext.KeyApostrophe)
	keySemicolon    gpu.Key = gpu.Key(gpucontext.KeySemicolon)
	keyLeftBracket  gpu.Key = gpu.Key(gpucontext.KeyLeftBracket)
	keyBackslash    gpu.Key = gpu.Key(gpucontext.KeyBackslash)
	keyRightBracket gpu.Key = gpu.Key(gpucontext.KeyRightBracket)
	keyGrave        gpu.Key = gpu.Key(gpucontext.KeyGrave)
	keyCapsLock     gpu.Key = gpu.Key(gpucontext.KeyCapsLock)
	keyScrollLock   gpu.Key = gpu.Key(gpucontext.KeyScrollLock)
	keyNumLock      gpu.Key = gpu.Key(gpucontext.KeyNumLock)
	keyKPDecimal    gpu.Key = gpu.Key(gpucontext.KeyNumpadDecimal)
	keyKPDivide     gpu.Key = gpu.Key(gpucontext.KeyNumpadDivide)
	keyKPMultiply   gpu.Key = gpu.Key(gpucontext.KeyNumpadMultiply)
	keyKPSubtract   gpu.Key = gpu.Key(gpucontext.KeyNumpadSubtract)
	keyKPAdd        gpu.Key = gpu.Key(gpucontext.KeyNumpadAdd)
	keyLeftShift    gpu.Key = gpu.Key(gpucontext.KeyLeftShift)
	keyLeftControl  gpu.Key = gpu.Key(gpucontext.KeyLeftControl)
	keyLeftAlt      gpu.Key = gpu.Key(gpucontext.KeyLeftAlt)
	keyRightShift   gpu.Key = gpu.Key(gpucontext.KeyRightShift)
	keyRightControl gpu.Key = gpu.Key(gpucontext.KeyRightControl)
	keyRightAlt     gpu.Key = gpu.Key(gpucontext.KeyRightAlt)
)

type window struct {
	done     <-chan struct{}
	held     map[gpu.Key]bool
	screen   *gpu.Image
	dw, dh   int
	buttons  [3]bool
	hidden   bool      // the OS pointer is hidden over the window
	lastDraw time.Time // the last frame shown, for the display's refresh interval
	r        *renderer // Hardware 3D (nil until the first frame, or after it failed)
	rFailed  bool
}

func (g *window) Update() error {
	select {
	case <-g.done:
		return gpu.ErrTermination
	default:
	}
	st.mu.Lock()
	quit := st.quit
	st.mu.Unlock()
	if quit {
		return gpu.ErrTermination
	}
	g.serve()
	if Scripted {
		return nil
	}
	if g.held == nil {
		g.held = map[gpu.Key]bool{}
	}
	for k, code := range scan {
		down := gpu.IsKeyPressed(k)
		if down == g.held[k] {
			continue
		}
		g.held[k] = down
		var brk byte
		if !down {
			brk = 0x80
		}
		if code>>8 == 0xE0 {
			PushKey(0xE0)
		}
		PushKey(byte(code) | brk)
	}
	g.mouse()
	return nil
}

// mouse hands the window's mouse to the game. The OS pointer, hidden over the window,
// places the game's cursor directly. While the game asks for relative motion (mouse look, a
// weapon swing: RequestPointerLock) the pointer is locked and its raw motion is sent;
// losing the focus or the game no longer asking releases it.
func (g *window) mouse() {
	st.mu.Lock()
	want := Now() < st.lockUntil
	warp, wx, wy := st.warp, st.ptrX, st.ptrY
	st.warp = false
	w, h := st.w, st.h
	st.mu.Unlock()
	want = want && gpu.IsFocused()
	if want != gpu.CursorCaptured() {
		gpu.SetCursorCaptured(want)
		gpu.CursorDelta() // the lock's own jump is not motion
		g.hidden = false
	}
	if gpu.CursorCaptured() {
		if dx, dy := gpu.CursorDelta(); dx != 0 || dy != 0 {
			MouseMove(dx, dy)
		}
	} else if g.dw > 0 && g.dh > 0 {
		if !g.hidden {
			gpu.SetCursorHidden(true)
			g.hidden = true
		}
		if warp {
			gpu.SetCursorPosition(int(wx*float64(g.dw)/float64(w)), int(wy*float64(g.dh)/float64(h)))
		} else {
			x, y := gpu.CursorPosition()
			MouseTo(float64(x)*float64(w)/float64(g.dw), float64(y)*float64(h)/float64(g.dh))
		}
	}
	st.mu.Lock()
	st.locked = gpu.CursorCaptured()
	st.mu.Unlock()
	for b, mb := range []gpu.MouseButton{gpu.MouseButtonLeft, gpu.MouseButtonRight, gpu.MouseButtonMiddle} {
		down := gpu.IsMouseButtonPressed(mb)
		if down != g.buttons[b] {
			g.buttons[b] = down
			MouseButton(b, down)
		}
	}
}

func (g *window) Draw(dst *gpu.Image) {
	g.serve()
	st.mu.Lock()
	px, w, h := rgbaLocked(), st.w, st.h
	shot := st.shot
	st.shot = ""
	scene, pal := st.scene, st.pal
	var frame []byte
	if scene != nil && g.r != nil && w == 320 && h == 200 {
		frame = append([]byte(nil), st.frame[:64000]...)
	}
	if g.screen == nil || g.screen.Bounds().Dx() != w || g.screen.Bounds().Dy() != h {
		g.screen = gpu.NewImage(w, h)
	}
	g.screen.WritePixels(px)
	if shot != "" && frame == nil {
		_ = WritePNG(shot, px, w, h)
	}
	now := time.Now()
	if !g.lastDraw.IsZero() {
		refreshNote(now.Sub(g.lastDraw))
	}
	g.lastDraw = now
	st.frameSeq++
	st.mu.Unlock()
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(float64(g.dw)/float64(w), float64(g.dh)/float64(h))
	dst.DrawImage(g.screen, op)
	if frame == nil {
		return
	}
	// Hardware 3D: the window-sized view over the frame
	shown := g.guard(func() bool { return g.r.composite(dst, scene, frame, pal) })
	if shot != "" {
		if !shown {
			_ = WritePNG(shot, px, w, h)
			return
		}
		dw, dh := dst.Bounds().Dx(), dst.Bounds().Dy()
		big := make([]byte, dw*dh*4)
		dst.ReadPixels(big)
		_ = WritePNG(shot, big, dw, dh)
	}
}

// serve draws a Hardware 3D request, if one waits.
func (g *window) serve() {
	if g.r == nil && !g.rFailed {
		r, err := newRenderer()
		if err != nil {
			fmt.Fprintln(os.Stderr, "dagger: hardware 3D:", err)
			g.rFailed = true
			return
		}
		g.r = r
		st.mu.Lock()
		r3d.live = true
		st.mu.Unlock()
	}
	if g.r == nil {
		return
	}
	select {
	case s := <-r3d.req:
		s.reply <- g.guard(func() bool { return g.r.render(s, g.dw, g.dh) })
	default:
	}
}

// guard runs a renderer call; a failure turns Hardware 3D off (the engine draws the view).
func (g *window) guard(f func() bool) (ok bool) {
	defer func() {
		if e := recover(); e != nil {
			fmt.Fprintln(os.Stderr, "dagger: hardware 3D:", e)
			gpu.Discard()
			g.r, g.rFailed, ok = nil, true, false
			st.mu.Lock()
			r3d.live = false
			st.mu.Unlock()
		}
	}()
	return f()
}

// Layout keeps the window's whole area, so the frame scales to it; the cursor maps through
// the same size.
func (g *window) Layout(ow, oh int) (int, int) {
	g.dw, g.dh = ow, oh
	return ow, oh
}
