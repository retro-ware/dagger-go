// Package music plays Daggerfall's General MIDI songs (HMI files from MIDI.BSA) the way the
// GOG release does through DOSBox's MPU-401: a General MIDI synthesizer, here meltysynth with
// a SoundFont read at startup (soundFont: $DAGGER_SF2, else the first .sf2 file in the
// working directory). Without a SoundFont there is no music.
package music

import (
	"bytes"
	"sync"

	"github.com/memmaker/dagger-go/internal/meltysynth"
)

// Rate is the sample rate the music renders at.
const Rate = 44100

var (
	mu     sync.Mutex
	synth  *meltysynth.Synthesizer
	song   *Song
	pos    int // next event
	at     int // samples since the song started
	volume = float32(1)
	loaded bool
)

// Init loads the SoundFont; false when there is none.
func Init() bool {
	mu.Lock()
	defer mu.Unlock()
	if loaded {
		return synth != nil
	}
	loaded = true
	sf2 := soundFont()
	if sf2 == nil {
		return false
	}
	sf, err := meltysynth.NewSoundFont(bytes.NewReader(sf2))
	if err != nil {
		return false
	}
	s, err := meltysynth.NewSynthesizer(sf, meltysynth.NewSynthesizerSettings(Rate))
	if err != nil {
		return false
	}
	synth = s
	return true
}

// Play starts an HMI song (nil or not a song: silence).
func Play(hmi []byte) {
	var s *Song
	if hmi != nil {
		s, _ = ParseHMI(hmi)
	}
	mu.Lock()
	defer mu.Unlock()
	song, pos, at = s, 0, 0
	if synth != nil {
		synth.NoteOffAll(false)
		synth.ResetAllControllers()
	}
}

// SetVolume sets the music volume, 0..128 (the game's slider).
func SetVolume(v int) {
	mu.Lock()
	volume = float32(min(max(v, 0), 128)) / 128
	mu.Unlock()
}

// Render adds the music's next len(l) samples to l and r. A song that ended starts again,
// as the game's music_update restarted it.
func Render(l, r []float32) {
	mu.Lock()
	defer mu.Unlock()
	if synth == nil {
		return
	}
	n := len(l)
	bl, br := make([]float32, n), make([]float32, n)
	out := 0
	for out < n {
		k := n - out
		if s := song; s != nil && s.Length > 0 {
			for pos < len(s.Events) && s.Events[pos].Tick*Rate/s.TicksPerSec <= at {
				e := s.Events[pos]
				synth.ProcessMidiMessage(int32(e.Status&15), int32(e.Status&0xF0), int32(e.D1), int32(e.D2))
				pos++
			}
			end := s.Length * Rate / s.TicksPerSec
			if at >= end {
				synth.NoteOffAll(false)
				pos, at = 0, 0
				continue
			}
			next := end
			if pos < len(s.Events) {
				next = s.Events[pos].Tick * Rate / s.TicksPerSec
			}
			k = min(k, next-at)
		}
		synth.Render(bl[out:out+k], br[out:out+k])
		at += k
		out += k
	}
	for i := range n {
		l[i] += bl[i] * volume
		r[i] += br[i] * volume
	}
}
