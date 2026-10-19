package host

import (
	"gpu"
)

// scan is a key's PC set-1 scan code; 0xE0 in the high byte marks a grey (extended) key.
var scan = map[gpu.Key]uint16{
	gpu.KeyEscape: 0x01, gpu.Key1: 0x02, gpu.Key2: 0x03, gpu.Key3: 0x04, gpu.Key4: 0x05,
	gpu.Key5: 0x06, gpu.Key6: 0x07, gpu.Key7: 0x08, gpu.Key8: 0x09, gpu.Key9: 0x0A,
	gpu.Key0: 0x0B, gpu.KeyMinus: 0x0C, gpu.KeyEqual: 0x0D, gpu.KeyBackspace: 0x0E,
	gpu.KeyTab: 0x0F, gpu.KeyQ: 0x10, gpu.KeyW: 0x11, gpu.KeyE: 0x12, gpu.KeyR: 0x13,
	gpu.KeyT: 0x14, gpu.KeyY: 0x15, gpu.KeyU: 0x16, gpu.KeyI: 0x17, gpu.KeyO: 0x18,
	gpu.KeyP: 0x19, glfwLeftBracket: 0x1A, glfwRightBracket: 0x1B, gpu.KeyEnter: 0x1C,
	glfwLeftControl: 0x1D, gpu.KeyA: 0x1E, gpu.KeyS: 0x1F, gpu.KeyD: 0x20, gpu.KeyF: 0x21,
	gpu.KeyG: 0x22, gpu.KeyH: 0x23, gpu.KeyJ: 0x24, gpu.KeyK: 0x25, gpu.KeyL: 0x26,
	glfwSemicolon: 0x27, glfwApostrophe: 0x28, glfwGrave: 0x29, glfwLeftShift: 0x2A,
	glfwBackslash: 0x2B, gpu.KeyZ: 0x2C, gpu.KeyX: 0x2D, gpu.KeyC: 0x2E, gpu.KeyV: 0x2F,
	gpu.KeyB: 0x30, gpu.KeyN: 0x31, gpu.KeyM: 0x32, gpu.KeyComma: 0x33, gpu.KeyPeriod: 0x34,
	gpu.KeySlash: 0x35, glfwRightShift: 0x36, glfwKPMultiply: 0x37, glfwLeftAlt: 0x38,
	gpu.KeySpace: 0x39, glfwCapsLock: 0x3A, gpu.KeyF1: 0x3B, gpu.KeyF2: 0x3C, gpu.KeyF3: 0x3D,
	gpu.KeyF4: 0x3E, gpu.KeyF5: 0x3F, gpu.KeyF6: 0x40, gpu.KeyF7: 0x41, gpu.KeyF8: 0x42,
	gpu.KeyF9: 0x43, gpu.KeyF10: 0x44, glfwNumLock: 0x45, glfwScrollLock: 0x46,
	gpu.KeyKP7: 0x47, gpu.KeyKP8: 0x48, gpu.KeyKP9: 0x49, glfwKPSubtract: 0x4A,
	gpu.KeyKP4: 0x4B, gpu.KeyKP5: 0x4C, gpu.KeyKP6: 0x4D, glfwKPAdd: 0x4E, gpu.KeyKP1: 0x4F,
	gpu.KeyKP2: 0x50, gpu.KeyKP3: 0x51, gpu.KeyKP0: 0x52, glfwKPDecimal: 0x53,
	gpu.KeyF11: 0x57, gpu.KeyF12: 0x58,
	gpu.KeyKPEnter: 0xE01C, glfwRightControl: 0xE01D, glfwKPDivide: 0xE035,
	glfwRightAlt: 0xE038, gpu.KeyHome: 0xE047, gpu.KeyUp: 0xE048, gpu.KeyPageUp: 0xE049,
	gpu.KeyLeft: 0xE04B, gpu.KeyRight: 0xE04D, gpu.KeyEnd: 0xE04F, gpu.KeyDown: 0xE050,
	gpu.KeyPageDown: 0xE051, gpu.KeyInsert: 0xE052, gpu.KeyDelete: 0xE053,
}

// GLFW's codes for the keys gpu has no names for.
const (
	glfwApostrophe   gpu.Key = 39
	glfwSemicolon    gpu.Key = 59
	glfwLeftBracket  gpu.Key = 91
	glfwBackslash    gpu.Key = 92
	glfwRightBracket gpu.Key = 93
	glfwGrave        gpu.Key = 96
	glfwCapsLock     gpu.Key = 280
	glfwScrollLock   gpu.Key = 281
	glfwNumLock      gpu.Key = 282
	glfwKPDecimal    gpu.Key = 330
	glfwKPDivide     gpu.Key = 331
	glfwKPMultiply   gpu.Key = 332
	glfwKPSubtract   gpu.Key = 333
	glfwKPAdd        gpu.Key = 334
	glfwLeftShift    gpu.Key = 340
	glfwLeftControl  gpu.Key = 341
	glfwLeftAlt      gpu.Key = 342
	glfwRightShift   gpu.Key = 344
	glfwRightControl gpu.Key = 345
	glfwRightAlt     gpu.Key = 346
)

// The window shows 320x200 with square pixels stretched to 4:3, as a VGA monitor showed it.
const aspect = 1.2

type window struct {
	done    <-chan struct{}
	held    map[gpu.Key]bool
	screen  *gpu.Image
	dw, dh  int
	buttons [3]bool
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

// mouse sends the window's mouse to the driver: motion while the window has the pointer
// captured (the first click captures it, Ctrl+G or losing focus releases it), and buttons.
func (g *window) mouse() {
	if gpu.CursorCaptured() && (!gpu.IsFocused() ||
		gpu.IsKeyPressed(gpu.KeyG) && (gpu.IsKeyPressed(glfwLeftControl) || gpu.IsKeyPressed(glfwRightControl))) {
		gpu.SetCursorCaptured(false)
	}
	dx, dy := gpu.CursorDelta()
	if !gpu.CursorCaptured() {
		if gpu.IsMouseButtonJustPressed(gpu.MouseButtonLeft) {
			gpu.SetCursorCaptured(true)
			gpu.CursorDelta()
		}
		return
	}
	st.mu.Lock()
	w, h := st.w, st.h
	st.mu.Unlock()
	if (dx != 0 || dy != 0) && g.dw > 0 && g.dh > 0 {
		MouseMove(dx*float64(w)/float64(g.dw), dy*float64(h)/float64(g.dh))
	}
	for b, mb := range []gpu.MouseButton{gpu.MouseButtonLeft, gpu.MouseButtonRight, gpu.MouseButtonMiddle} {
		down := gpu.IsMouseButtonPressed(mb)
		if down != g.buttons[b] {
			g.buttons[b] = down
			MouseButton(b, down)
		}
	}
}

func (g *window) Draw(dst *gpu.Image) {
	st.mu.Lock()
	px, w, h := rgbaLocked(), st.w, st.h
	shot := st.shot
	st.shot = ""
	if g.screen == nil || g.screen.Bounds().Dx() != w || g.screen.Bounds().Dy() != h {
		g.screen = gpu.NewImage(w, h)
	}
	g.screen.WritePixels(px)
	if shot != "" {
		_ = WritePNG(shot, px, w, h)
	}
	st.mu.Unlock()
	op := &gpu.DrawImageOptions{}
	op.GeoM.Scale(float64(g.dw)/float64(w), float64(g.dh)/float64(h))
	dst.DrawImage(g.screen, op)
}

// Layout keeps the window's whole area, so the frame scales to it; the cursor maps through
// the same size.
func (g *window) Layout(ow, oh int) (int, int) {
	g.dw, g.dh = ow, oh
	return ow, oh
}
