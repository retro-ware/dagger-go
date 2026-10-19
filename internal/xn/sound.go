package xn

import "github.com/memmaker/dagger-go/internal/host"

// The digital sound boundary (the game's SOS driver as XnGine and the game use it). The host
// sets the functions; without them there is no digital device. A sample's Done runs from
// Poll, on the game's goroutine, as the driver's callback ran between the game's
// instructions.

// Sample is a sample to play: 8-bit unsigned mono PCM at Rate Hz. When its Done gives it new
// Data, the device plays that next, as the driver chained a sample its callback refilled.
type Sample struct {
	Data   []byte
	Rate   int
	Volume int  // 0..0x7F
	Pan    int  // 0 left .. 0x8000 centre .. 0xFFFF right
	Loop   bool // play again from the start until stopped
	Done   func(s *Sample)
}

var (
	// SoundDigiDevice is the digital device: -1 none.
	SoundDigiDevice int32 = -1
	// SoundStart starts s; its handle, or -1.
	SoundStart = func(s *Sample) int { return -1 }
	// SoundStop stops a sample (its Done does not run).
	SoundStop = func(h int) {}
	// SoundEnded returns the handles of the samples that have ended since the last call.
	SoundEnded = func() []int { return nil }
	// SoundRequeue plays data next on the ended sample h.
	SoundRequeue = func(h int, data []byte) {}
)

// soundPlaying is the samples started, by handle.
var soundPlaying = map[int]*Sample{}

// SoundUseHost makes the host's sound card the digital device.
func SoundUseHost() {
	if !host.AudioInit() {
		return
	}
	SoundDigiDevice = 0
	SoundStart = func(s *Sample) int {
		h := host.AudioStart(host.Voice{Data: s.Data, Rate: s.Rate, Volume: s.Volume, Pan: s.Pan, Loop: s.Loop, Chained: s.Done != nil})
		soundPlaying[h] = s
		return h
	}
	SoundStop = func(h int) {
		if h >= 0 {
			host.AudioStop(h)
			delete(soundPlaying, h)
		}
	}
	SoundEnded = host.AudioEnded
	SoundRequeue = host.AudioRequeue
}

func runSoundCallbacks() {
	for _, h := range SoundEnded() {
		s := soundPlaying[h]
		if s == nil {
			continue
		}
		old := s.Data
		if s.Done != nil {
			s.Done(s)
		}
		if soundPlaying[h] != s {
			continue // stopped by its callback
		}
		if len(s.Data) > 0 && (len(old) == 0 || &s.Data[0] != &old[0] || len(s.Data) != len(old)) {
			SoundRequeue(h, s.Data)
			continue
		}
		SoundStop(h)
	}
}
