package host

import (
	"sync"

	"github.com/ebitengine/oto/v3"
)

// The sound card: voices of 8-bit unsigned mono PCM mixed to 44.1 kHz stereo through oto.
// A voice that ends stays allocated until the game takes it back (Requeue or Stop), so a
// game callback can chain the next buffer on the same handle.

const mixRate = 44100

// Voice is a sample playing.
type Voice struct {
	Data   []byte
	Rate   int
	Volume int  // 0..0x7F
	Pan    int  // 0 left .. 0x8000 centre .. 0xFFFF right
	Loop   bool // play again from the start until stopped
}

type voice struct {
	Voice
	pos   uint64 // 32.32 fixed position in Data
	ended bool
	used  bool
}

var mix struct {
	mu     sync.Mutex
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
	mix.player = ctx.NewPlayer(mixer{})
	mix.player.SetBufferSize(mixRate / 30 * 4)
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

// AudioRequeue plays data next on an ended voice h.
func AudioRequeue(h int, data []byte) {
	mix.mu.Lock()
	defer mix.mu.Unlock()
	if h >= 0 && h < len(mix.voices) && mix.voices[h].used {
		v := &mix.voices[h]
		v.Data, v.pos, v.ended = data, 0, false
	}
}

// AudioStop frees voice h.
func AudioStop(h int) {
	mix.mu.Lock()
	defer mix.mu.Unlock()
	if h >= 0 && h < len(mix.voices) {
		mix.voices[h] = voice{}
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

func (mixer) Read(b []byte) (int, error) {
	n := len(b) / 4
	mix.mu.Lock()
	defer mix.mu.Unlock()
	for i := 0; i < n; i++ {
		var l, r int32
		for h := range mix.voices {
			v := &mix.voices[h]
			if !v.used || v.ended || v.Rate <= 0 {
				continue
			}
			idx := int(v.pos >> 32)
			if idx >= len(v.Data) {
				if v.Loop && len(v.Data) > 0 {
					v.pos, idx = 0, 0
				} else {
					v.ended = true
					mix.ended = append(mix.ended, h)
					continue
				}
			}
			s := (int32(v.Data[idx]) - 128) * int32(v.Volume) // 8+7 bits
			pan := int32(v.Pan >> 8)                          // 0..255
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
