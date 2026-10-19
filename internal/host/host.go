// Package host is the desktop around the game: a window that shows the game's 8-bit frames
// through its VGA palette, the keyboard as PC scan codes, the mouse, and a clock. The game
// runs on its own goroutine (Run) and talks to the host only through this package's calls,
// which are safe to make from that goroutine.
package host

import (
	"image"
	"image/png"
	"os"
	"sync"
	"time"

	"gpu"
)

// The largest mode the game sets (VESA 640x480); mode 13h is 320x200.
const (
	MaxWidth  = 640
	MaxHeight = 480
)

// A Color is a VGA DAC entry: 6 bits each of red, green and blue.
type Color struct{ R, G, B uint8 }

var st struct {
	mu      sync.Mutex
	w, h    int                        // the presented frame's size
	frame   [MaxWidth * MaxHeight]byte // the presented frame, palette indices
	pal     [256]Color
	rgba    []byte
	keys    []byte // scan codes not yet read: make codes, and break codes with bit 7 set
	mx, my  int    // the mouse, in the frame's pixels
	buttons int    // bit 0 left, bit 1 right, bit 2 middle
	quit    bool
	start   time.Time
	shot    string // a PNG to write of the next frame shown
}

func init() { st.w, st.h, st.start = 320, 200, time.Now() }

// Present shows a frame of w x h palette indices.
func Present(px []byte, w, h int) {
	st.mu.Lock()
	st.w, st.h = w, h
	copy(st.frame[:w*h], px)
	st.mu.Unlock()
}

// SetPalette sets DAC entries first.. from cs.
func SetPalette(first int, cs []Color) {
	st.mu.Lock()
	copy(st.pal[first:], cs)
	st.mu.Unlock()
}

// Palette returns the DAC.
func Palette() [256]Color {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.pal
}

// ReadKey returns the next scan code the keyboard sent, or false.
func ReadKey() (byte, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if len(st.keys) == 0 {
		return 0, false
	}
	k := st.keys[0]
	st.keys = st.keys[1:]
	return k, true
}

// PushKey queues a scan code, as a script or the window sends one.
func PushKey(code byte) {
	st.mu.Lock()
	st.keys = append(st.keys, code)
	st.mu.Unlock()
}

// Mouse returns the cursor in the frame's pixels and the buttons held.
func Mouse() (x, y, buttons int) {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.mx, st.my, st.buttons
}

// SetMouse moves the cursor and sets the buttons, as a script does.
func SetMouse(x, y, buttons int) {
	st.mu.Lock()
	st.mx, st.my, st.buttons = x, y, buttons
	st.mu.Unlock()
}

// Quit asks the window to close.
func Quit() {
	st.mu.Lock()
	st.quit = true
	st.mu.Unlock()
}

// Now is the time since the program started; a deterministic run replaces it.
var Now = func() time.Duration { return time.Since(st.start) }

// Sleep waits a little: the game's busy waits call it so they don't spin.
var Sleep = func() { time.Sleep(time.Millisecond) }

// Screenshot writes the next frame shown to a PNG.
func Screenshot(path string) {
	st.mu.Lock()
	st.shot = path
	st.mu.Unlock()
}

// FrameRGBA returns the presented frame as RGBA, with its size.
func FrameRGBA() ([]byte, int, int) {
	st.mu.Lock()
	defer st.mu.Unlock()
	return append([]byte(nil), rgbaLocked()...), st.w, st.h
}

func rgbaLocked() []byte {
	n := st.w * st.h
	if len(st.rgba) != n*4 {
		st.rgba = make([]byte, n*4)
	}
	var lut [256][4]byte
	for i, c := range st.pal {
		lut[i] = [4]byte{c.R<<2 | c.R>>4, c.G<<2 | c.G>>4, c.B<<2 | c.B>>4, 0xFF}
	}
	for i, p := range st.frame[:n] {
		copy(st.rgba[i*4:], lut[p][:])
	}
	return st.rgba
}

// WritePNG writes RGBA pixels to a PNG file.
func WritePNG(path string, px []byte, w, h int) error {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	copy(img.Pix, px)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// Run opens the window and runs game on its own goroutine until it returns or the window
// closes.
func Run(title string, game func()) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		game()
	}()
	gpu.SetWindowTitle(title)
	gpu.SetWindowSize(960, 600)
	gpu.SetWindowResizingMode(gpu.WindowResizingModeEnabled)
	gpu.SetTPS(70)
	gpu.SetRunnableOnUnfocused(true)
	return gpu.RunGame(&window{done: done})
}

// Headless runs game without a window (a scripted run, or a test).
func Headless(game func()) { game() }
