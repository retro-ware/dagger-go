package xn

import (
	"time"

	"github.com/memmaker/dagger-go/internal/host"
)

// Timing (timer.c) and the interrupts behind it: the BIOS tick at 18.2 Hz, and the periodic
// callbacks the game registers with the sound system's timer (SOS's sosTIMERRegisterEvent),
// all run from Poll when their time has come.

const pitHz = 1193182

var (
	TimerTickCount uint32 // counted by TimerTickCallback
	FrameTimerMs   uint32
	TimerFPS       uint32
	FPSAverage     uint32
	fpsFrameCount  uint32
	fpsLastTick    uint32
)

// BIOSTicks is the BIOS tick count at 0x46C.
func BIOSTicks() uint32 {
	return uint32(uint64(host.Now()) * pitHz / 65536 / uint64(time.Second))
}

// A periodic callback at Rate Hz.
type timerEvent struct {
	rate  int
	fn    func()
	fired uint64 // calls made since registration
	start time.Duration
	live  bool
}

var timers []*timerEvent

// TimerRegister adds fn to run rate times a second; it returns a handle.
func TimerRegister(rate int, fn func()) int {
	e := &timerEvent{rate: rate, fn: fn, start: host.Now(), live: true}
	for i, t := range timers {
		if !t.live {
			timers[i] = e
			return i
		}
	}
	timers = append(timers, e)
	return len(timers) - 1
}

// TimerRemove stops a callback.
func TimerRemove(h int) {
	if h >= 0 && h < len(timers) {
		timers[h].live = false
	}
}

var inTimers bool

func runTimers() {
	if inTimers {
		return
	}
	inTimers = true
	now := host.Now()
	for _, t := range timers {
		if !t.live {
			continue
		}
		due := uint64(now-t.start) * uint64(t.rate) / uint64(time.Second)
		for t.live && t.fired < due {
			t.fired++
			t.fn()
		}
	}
	inTimers = false
}

func TimerTickCallback() { TimerTickCount++ }
func TimerAdd25()        { FrameTimerMs += 25 }

// TimerFPSUpdate counts a frame; once a second (more than 18 ticks) it updates the rate.
func TimerFPSUpdate() uint32 {
	fpsFrameCount++
	if BIOSTicks()-fpsLastTick > 18 {
		fpsLastTick = BIOSTicks()
		TimerFPS = fpsFrameCount
		FPSAverage = (TimerFPS + FPSAverage) >> 1
		fpsFrameCount = 0
	}
	return uint32(uint16(TimerFPS))
}

// TimerWaitTicks waits n BIOS ticks.
func TimerWaitTicks(n uint32) {
	start := BIOSTicks()
	for BIOSTicks()-start < n {
		Poll()
		idle()
	}
}

// TimerWaitChange waits until *counter differs from *last, then records it.
func TimerWaitChange(last *int32, counter *int32) {
	for *counter == *last {
		Poll()
		idle()
	}
	*last = *counter
}
