package xn

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
	// SoundPending is how many samples have ended whose Done has not run: Poll runs them.
	SoundPending = func() []*Sample { return nil }
)

func runSoundCallbacks() {
	for _, s := range SoundPending() {
		if s.Done != nil {
			s.Done(s)
		}
	}
}
