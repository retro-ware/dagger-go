package host

import (
	"math"
	"time"
)

// Frame pacing. The game draws a whole frame at a time on its goroutine and the window shows
// the latest one at each refresh of the display (vsync). A game frame begins every n-th
// refresh, n the smallest that keeps the game at most maxGameRate frames a second: 60 Hz
// and 75 Hz displays get a game frame each refresh, 120 Hz and 144 Hz every second, 240 Hz
// every fourth. So each game frame stays on screen equally long (no judder), and the game's
// own per-frame steps run near the 70 a second the port was tuned at. Without frames being
// shown (no window, minimised) the game runs at about fallbackRate.
const (
	maxGameRate  = 75
	fallbackRate = 70
)

var pace struct {
	interval  time.Duration // the display's measured refresh interval
	shown     uint64        // frames shown when the last game frame began
	began     time.Duration // when it began (Now)
	presented uint64        // frames shown when the game last presented
	presentAt time.Duration // when it did (Now)
}

// refreshNote folds a measured interval between shown frames into the estimate (st.mu
// held). Intervals far off a display's (a stall, an occluded window spinning) are ignored.
func refreshNote(d time.Duration) {
	if d < 2*time.Millisecond || d > 50*time.Millisecond {
		return
	}
	if pace.interval == 0 {
		pace.interval = d
		return
	}
	pace.interval += (d - pace.interval) / 16
}

// RefreshRate is the display's measured refresh rate in Hz (0 before it is known).
func RefreshRate() float64 {
	st.mu.Lock()
	defer st.mu.Unlock()
	if pace.interval == 0 {
		return 0
	}
	return float64(time.Second) / float64(pace.interval)
}

// GameFrameDue is whether the game's next frame should begin, and starts its wait if so.
func GameFrameDue() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	n := uint64(1)
	iv := time.Second / fallbackRate
	if pace.interval != 0 {
		iv = pace.interval
		hz := float64(time.Second) / float64(pace.interval)
		n = uint64(max(1, math.Ceil(hz/maxGameRate-0.05)))
	}
	since := Now() - pace.began
	if since < time.Second/(maxGameRate+8) {
		return false // never faster than the cap, however the display behaves
	}
	if st.frameSeq < pace.shown+n && since < time.Duration(n)*iv*3/2 {
		return false
	}
	pace.shown, pace.began = st.frameSeq, Now()
	return true
}

// presentPace holds a present that changes nothing on screen until the window has shown a
// frame since the last present (st.mu held, released while it waits), so the game's screens
// with a loop of their own (the save menu, the message boxes), which present each time round,
// go round at the display's pace instead of 190,000 times a second. A frame that changes the
// screen goes at once, so nothing the game draws waits.
func presentPace(changed bool) {
	if !changed {
		iv := pace.interval
		if iv == 0 {
			iv = time.Second / fallbackRate
		}
		for st.frameSeq == pace.presented && Now()-pace.presentAt < iv*3/2 {
			st.mu.Unlock()
			Sleep()
			st.mu.Lock()
		}
	}
	pace.presented, pace.presentAt = st.frameSeq, Now()
}
