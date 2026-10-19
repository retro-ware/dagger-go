package host

import (
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

// The sound card: voices of 8-bit unsigned mono PCM mixed to 44.1 kHz stereo through oto.
// A voice that ends stays allocated until the game takes it back (Requeue or Stop), so a
// game callback can chain the next buffer on the same handle.
//
// On DOS the sound driver called a sample's done callback from its interrupt, right at the
// sample's end, so a chained buffer followed without a break. oto pulls audio in chunks, and
// the game answers ended voices only when it polls; so the mixer stops a chunk where a chained
// voice ends and waits (briefly) for the game to requeue or stop it before mixing on.

const (
	mixRate = 44100
	// chainWait is how long the mixer waits for the game to chain a buffer; the player's
	// buffer covers it.
	chainWait = 25 * time.Millisecond
)

// Voice is a sample playing.
type Voice struct {
	Data    []byte
	Rate    int
	Volume  int  // 0..0x7F
	Pan     int  // 0 left .. 0x8000 centre .. 0xFFFF right
	Loop    bool // play again from the start until stopped
	Chained bool // the game may requeue the voice at its end: mixing waits for it
}

type voice struct {
	Voice
	pos     uint64 // 32.32 fixed position in Data
	ended   bool
	waiting bool // chained and ended: the game has not answered yet
	used    bool
}

var mix struct {
	mu     sync.Mutex
	cond   *sync.Cond
	voices []voice
	ended  []int
	player *oto.Player
}

// AudioInit opens the sound card; false when there is none.
func AudioInit() bool {
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{SampleRate: mixRate, ChannelCount: 2, Format: oto.FormatSignedInt16LE})
	if err != nil {
		return false
	}
	<-ready
	mix.cond = sync.NewCond(&mix.mu)
	mix.player = ctx.NewPlayer(mixer{})
	mix.player.SetBufferSize(mixRate / 15 * 4)
	mix.player.Play()
	return true
}

// AudioStart plays v; its handle.
func AudioStart(v Voice) int {
	mix.mu.Lock()
	defer mix.mu.Unlock()
	for i := range mix.voices {
		if !mix.voices[i].used {
			mix.voices[i] = voice{Voice: v, used: true}
			return i
		}
	}
	mix.voices = append(mix.voices, voice{Voice: v, used: true})
	return len(mix.voices) - 1
}

// AudioRequeue plays data next on an ended voice h, from where the last buffer left off.
func AudioRequeue(h int, data []byte) {
	mix.mu.Lock()
	defer mix.mu.Unlock()
	if h >= 0 && h < len(mix.voices) && mix.voices[h].used {
		v := &mix.voices[h]
		end := uint64(len(v.Data)) << 32
		if v.pos >= end {
			v.pos -= end // keep the fraction: the new buffer continues the old
		} else {
			v.pos = 0
		}
		v.Data, v.ended, v.waiting = data, false, false
		mix.cond.Broadcast()
	}
}

// AudioStop frees voice h.
func AudioStop(h int) {
	mix.mu.Lock()
	defer mix.mu.Unlock()
	if h >= 0 && h < len(mix.voices) {
		mix.voices[h] = voice{}
		mix.cond.Broadcast()
	}
}

// AudioSet changes voice h's volume (0..0x7F) and pan.
func AudioSet(h, volume, pan int) {
	mix.mu.Lock()
	defer mix.mu.Unlock()
	if h >= 0 && h < len(mix.voices) && mix.voices[h].used {
		mix.voices[h].Volume, mix.voices[h].Pan = volume, pan
	}
}

// AudioEnded returns the voices that have ended since the last call.
func AudioEnded() []int {
	mix.mu.Lock()
	defer mix.mu.Unlock()
	e := mix.ended
	mix.ended = nil
	return e
}

type mixer struct{}

// anyWaiting is whether a chained voice waits for the game. mix.mu is held.
func anyWaiting() bool {
	for i := range mix.voices {
		if mix.voices[i].waiting {
			return true
		}
	}
	return false
}

// waitChains waits until no voice waits for the game, or chainWait passes (then those voices
// stay silent until the game answers). mix.mu is held.
func waitChains() {
	if !anyWaiting() {
		return
	}
	deadline := time.Now().Add(chainWait)
	t := time.AfterFunc(chainWait, func() {
		mix.mu.Lock()
		mix.cond.Broadcast()
		mix.mu.Unlock()
	})
	defer t.Stop()
	for anyWaiting() && time.Now().Before(deadline) {
		mix.cond.Wait()
	}
	for i := range mix.voices {
		mix.voices[i].waiting = false
	}
}

// sample is v's sample at its position, linearly interpolated (the card's output filter
// smoothed the steps).
func (v *voice) sample(idx int) int32 {
	s0 := int32(v.Data[idx])
	s1 := s0
	if idx+1 < len(v.Data) {
		s1 = int32(v.Data[idx+1])
	} else if v.Loop {
		s1 = int32(v.Data[0])
	}
	f := int32(v.pos >> 16 & 0xFFFF)
	return (s0*(0x10000-f)+s1*f)>>16 - 128
}

func (mixer) Read(b []byte) (int, error) {
	n := len(b) / 4
	mix.mu.Lock()
	defer mix.mu.Unlock()
	waitChains()
	for i := 0; i < n; i++ {
		// the ends first, so a chunk cut at a chained end has every voice at the same sample
		for h := range mix.voices {
			v := &mix.voices[h]
			if !v.used || v.ended || v.Rate <= 0 || int(v.pos>>32) < len(v.Data) {
				continue
			}
			if v.Loop && len(v.Data) > 0 {
				for int(v.pos>>32) >= len(v.Data) {
					v.pos -= uint64(len(v.Data)) << 32
				}
				continue
			}
			v.ended = true
			mix.ended = append(mix.ended, h)
			if v.Chained {
				v.waiting = true
			}
		}
		if anyWaiting() {
			if i > 0 {
				return i * 4, nil // the game chains the next buffer before the chunk goes on
			}
			waitChains()
			i--
			continue
		}
		var l, r int32
		for h := range mix.voices {
			v := &mix.voices[h]
			if !v.used || v.ended || v.Rate <= 0 {
				continue
			}
			s := v.sample(int(v.pos>>32)) * int32(v.Volume) // 8+7 bits
			pan := int32(v.Pan >> 8)                        // 0..255
			l += s * (255 - pan) >> 7
			r += s * pan >> 7
			v.pos += uint64(v.Rate) << 32 / mixRate
		}
		l, r = clamp16(l), clamp16(r)
		b[i*4], b[i*4+1] = byte(l), byte(l>>8)
		b[i*4+2], b[i*4+3] = byte(r), byte(r>>8)
	}
	return n * 4, nil
}
func clamp16(v int32) int32 { return max(-32768, min(32767, v)) }
