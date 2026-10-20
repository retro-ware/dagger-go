package xn

// The VID movie player (vid.c): a VID file streamed through a 64000-byte read buffer, its
// chunks decoded into the clip window (raw, delta, RLE frames; palettes; 8-bit audio
// streamed through two buffers into the digital sound device), timed by a 60 Hz timer or,
// while streaming audio, by the samples' ends.
//
// Positions in the buffer are offsets. Dropped: Q-VID-03 (an unopened file read as handle
// 0: here the movie ends), Q-VID-04 (a raw frame's rows after a refill take the DOS file
// handle as their width: here they keep the width), D-VID-01 (the region locks).

import (
	"encoding/binary"
	"io"
	"os"

	"github.com/retro-ware/dagger-go/internal/dosfs"
)

const (
	vidBuffer       = 0xFA00
	vidRefillMargin = 0x81
	vidAudioBuffer  = 0x4000
	vidTimerHz      = 60

	vidRaw       = 0
	vidDelta     = 1
	vidPalette   = 2
	vidRLE       = 3
	vidDeltaFrom = 4
	vidAudioHead = 0x7C
	vidAudio     = 0x7D
)

// VidHeader is a VID file's header.
type VidHeader struct {
	Magic      [4]byte
	Version    uint8 // a row-end byte follows rows when >= 2
	Frames     uint16
	Width      uint16
	Height     uint16
	FrameDelay uint16 // added to every frame's own delay (60 Hz ticks)
	Flags      uint16 // bit 0: leave the file open at the end
}

// VidPlayer is the player's state.
type VidPlayer struct {
	X, Y          int32
	Skippable     bool
	file          *os.File
	FilePos       int32
	buf           []byte
	ReadPtr       int32
	refillMark    int32
	DestOffset    int32
	DestSkip      int32
	Flags         uint16
	FramesLeft    uint16
	FramesDone    uint16
	RowWidth      uint32
	Rows          uint32
	RowsLeft      uint32
	FrameTicks    int16
	FrameWaits    uint8
	Done          uint8
	timer         int
	Palette       [0x300]byte
	AudioState    uint8 // 0 none, 1 header read, 2 waiting its delay, 3 streaming
	AudioDelay    uint16
	AudioTicks    uint16
	AudioWaiting  uint8
	RateByte      uint8
	SampleRate    uint16
	AudioSize     uint16
	AudioLeft     uint16
	AudioBufA     []byte
	AudioBufB     []byte
	AudioLenA     uint16
	AudioLenB     uint16
	AudioBuffer   uint8 // the buffer the next audio chunk fills
	SampleDone    uint8
	SampleRestart uint8
	sampleHandle  int
}

var (
	VidHeaderData VidHeader
	VidState      VidPlayer
	vidSample     Sample
)

func (v *VidPlayer) at(p int32) byte {
	if p >= 0 && int(p) < len(v.buf) {
		return v.buf[p]
	}
	return 0
}

func (v *VidPlayer) word(p int32) uint16 { return uint16(v.at(p)) | uint16(v.at(p+1))<<8 }

func screenSet(i int32, c byte) {
	if i >= 0 && int(i) < len(Screen) {
		Screen[i] = c
	}
}

// vidShow shows the opened movie until it ends (or a button or key when skippable), then
// sets its palette.
func vidShow() {
	v := &VidState
	ClipLeft, ClipRight = v.X, v.X+int32(VidHeaderData.Width)
	ClipTop, ClipBottom = v.Y, v.Y+int32(VidHeaderData.Height)
	Present(true)
	for {
		if v.Skippable {
			MousePollClamped()
			if MouseButtons&3 != 0 {
				break
			}
			if uint8(ReadKey()) != 0 {
				break
			}
		}
		VidUpdate()
		Present(true)
		if v.Done == 1 {
			break
		}
	}
	VidFinish()
	PalSet(v.Palette[:])
}

// VidPlay plays the movie at path at (x, y); false when it is not there.
func VidPlay(path string, x, y int32, skippable bool) bool {
	v := &VidState
	left, right, top, bottom := ClipLeft, ClipRight, ClipTop, ClipBottom
	v.Skippable = skippable
	played := false
	if dosfs.Exists(path) && VidOpen(x, y, path) && v.timer != -1 {
		vidShow()
		played = true
	}
	if !played {
		v.AudioBufA, v.AudioBufB = nil, nil
	}
	ClipLeft, ClipRight, ClipTop, ClipBottom = left, right, top, bottom
	return played
}

// VidOpen opens the movie and decodes its first chunk.
func VidOpen(x, y int32, path string) bool {
	v := &VidState
	v.FilePos = 0
	v.X, v.Y = x, y
	v.AudioState = 0
	v.buf = BigBuffer[:vidBuffer]
	v.refillMark = vidBuffer - vidRefillMargin
	v.AudioBufA = make([]byte, vidAudioBuffer)
	v.AudioBufB = make([]byte, vidAudioBuffer)
	v.AudioBuffer, v.SampleDone, v.SampleRestart = 0, 0, 0
	vidSample = Sample{}
	v.AudioTicks, v.FrameTicks = 0, 0
	v.timer = TimerRegister(vidTimerHz, VidTimerCallback)
	f, err := dosfs.Open(path)
	if err != nil {
		VidFinish()
		return false
	}
	v.file = f
	var h [15]byte
	io.ReadFull(f, h[:])
	hd := &VidHeaderData
	copy(hd.Magic[:], h[:4])
	hd.Version = h[4]
	hd.Frames = binary.LittleEndian.Uint16(h[5:])
	hd.Width = binary.LittleEndian.Uint16(h[7:])
	hd.Height = binary.LittleEndian.Uint16(h[9:])
	hd.FrameDelay = binary.LittleEndian.Uint16(h[11:])
	hd.Flags = binary.LittleEndian.Uint16(h[13:])
	v.DestOffset = RowOffset[v.Y] + v.X
	v.DestSkip = Width - int32(hd.Width)
	v.Flags = hd.Flags
	v.FramesLeft = hd.Frames
	v.RowWidth = uint32(hd.Width)
	v.Rows = uint32(hd.Height)
	v.FramesDone, v.FrameWaits = 0, 0
	VidFill()
	v.Done = 0
	v.FrameTicks = 0
	VidDecodeChunk()
	return true
}

func VidUpdate() {
	v := &VidState
	if SoundDigiDevice != -1 && v.AudioState == 3 {
		// streaming: the next frame comes when the sample has played
		if v.SampleDone != 1 {
			Poll()
			return
		}
		v.SampleDone = 0
		if v.SampleRestart == 1 {
			VidAudioStart()
			v.SampleRestart = 0
		}
	} else if v.FrameTicks > 0 {
		Poll()
		return
	}
	if v.FrameWaits != 0 {
		v.FrameWaits, v.SampleDone, v.AudioBuffer = 0, 0, 0
		VidAudioStop()
	}
	VidDecodeChunk()
}

func VidFinish() {
	v := &VidState
	VidAudioStop()
	v.AudioBufA, v.AudioBufB = nil, nil
	if v.Flags&1 == 0 {
		VidCloseFile()
	}
	v.Done = 1
	TimerRemove(v.timer)
}

// ---- decoding ----

func (v *VidPlayer) audioHeader(p int32) {
	v.AudioDelay = v.word(p)
	v.RateByte = v.at(p + 2)
	v.SampleRate = uint16(1000000 / (0x100 - uint32(v.RateByte)))
	VidReadAudio(p + 3)
	v.AudioState = 1
	v.AudioTicks = v.AudioDelay
	if v.AudioDelay != 0 {
		v.AudioWaiting = 0
	}
}

func (v *VidPlayer) palette(p int32) {
	if p > vidBuffer-0x300 {
		p = VidRefill(p)
	}
	for i := int32(0); i < 0x300; i++ {
		v.Palette[i] = v.at(p + i)
	}
	v.ReadPtr = p + 0x300
	for i := 0; i < 256; i++ {
		dacWrite(i, v.Palette[3*i], v.Palette[3*i+1], v.Palette[3*i+2])
	}
	dacFlush()
}

func (v *VidPlayer) rowDone(dst *int32, w *uint32) bool {
	v.RowsLeft--
	if v.RowsLeft == 0 {
		return true
	}
	*dst += v.DestSkip
	*w = v.RowWidth
	return false
}

func (v *VidPlayer) copyOut(dst, p int32, n uint32) {
	for k := int32(0); k < int32(n); k++ {
		screenSet(dst+k, v.at(p+k))
	}
}

// delta runs a delta frame's commands: 80h + n skips n pixels, n < 80h copies n bytes, 0
// ends the frame; runs carry over row ends.
func (v *VidPlayer) delta(p, dst int32, w uint32, refillFirst bool) int32 {
	rowEnd := func() int32 {
		if VidHeaderData.Version >= 2 {
			p++
		}
		return p
	}
	for {
		if refillFirst && p > v.refillMark {
			p = VidRefill(p)
		}
		refillFirst = true
		b := v.at(p)
		p++
		if b == 0 {
			return p
		}
		n := uint32(b & 0x7F)
		if b&0x80 != 0 {
			if n == 0 {
				continue
			}
			for n >= w {
				dst += int32(w)
				n -= w
				if v.rowDone(&dst, &w) {
					return rowEnd()
				}
			}
			dst += int32(n)
			w -= n
			continue
		}
		for n >= w {
			m := w
			v.copyOut(dst, p, m)
			dst += int32(m)
			p += int32(m)
			if v.rowDone(&dst, &w) {
				return rowEnd()
			}
			n -= m
			if n == 0 {
				break
			}
		}
		if n != 0 && n < w {
			v.copyOut(dst, p, n)
			dst += int32(n)
			p += int32(n)
			w -= n
		}
	}
}

// rle runs an RLE frame's commands: 80h + n then a colour fills n pixels, n < 80h copies n
// bytes; the frame ends after its last row.
func (v *VidPlayer) rle(p, dst int32, w uint32) int32 {
	for {
		if p > v.refillMark {
			p = VidRefill(p)
		}
		b := v.at(p)
		p++
		n := uint32(b & 0x7F)
		if n == 0 {
			continue
		}
		if b&0x80 != 0 {
			colour := v.at(p)
			p++
			for n >= w {
				m := w
				for ; w != 0; w-- {
					screenSet(dst, colour)
					dst++
				}
				if v.rowDone(&dst, &w) {
					return p
				}
				n -= m
				if n == 0 {
					break
				}
			}
			if n != 0 && n < w {
				w -= n
				for ; n != 0; n-- {
					screenSet(dst, colour)
					dst++
				}
			}
			continue
		}
		for n >= w {
			m := w
			v.copyOut(dst, p, m)
			dst += int32(m)
			p += int32(m)
			if v.rowDone(&dst, &w) {
				return p
			}
			n -= m
			if n == 0 {
				break
			}
		}
		if n != 0 && n < w {
			v.copyOut(dst, p, n)
			dst += int32(n)
			p += int32(n)
			w -= n
		}
	}
}

// VidDecodeChunk decodes the next frame (with the audio chunks after it), or a palette or
// audio chunk.
func VidDecodeChunk() {
	v := &VidState
	for {
		p := v.ReadPtr
		dst := v.DestOffset
		v.RowsLeft = v.Rows
		w := v.RowWidth
		typ := v.at(p)
		p++
		switch typ {
		case vidDeltaFrom:
			VidScheduleFrame(v.word(p))
			start := uint32(v.word(p + 2))
			p += 4
			v.RowsLeft -= start
			if v.RowsLeft != 0 {
				p = v.delta(p, dst+RowOffset[start], w, true)
			}
		case vidDelta:
			VidScheduleFrame(v.word(p))
			p = v.delta(p+2, dst, w, false)
		case vidRLE:
			VidScheduleFrame(v.word(p))
			p = v.rle(p+2, dst, w)
		case vidRaw:
			VidScheduleFrame(v.word(p))
			p += 2
			for {
				if vidBuffer-int32(v.RowWidth) < p {
					p = VidRefill(p)
				}
				v.copyOut(dst, p, w)
				dst += int32(w) + v.DestSkip
				p += int32(w)
				v.RowsLeft--
				if v.RowsLeft == 0 {
					break
				}
			}
		case vidAudioHead:
			v.audioHeader(p)
			return
		case vidAudio:
			VidReadAudio(p)
			return
		case vidPalette:
			v.palette(p)
			return
		default:
			VidFinish()
			return
		}
		// the frame is done
		p = VidRefill(p)
		next := v.at(p)
		v.ReadPtr = p
		v.FramesDone++
		v.FramesLeft--
		if v.FramesLeft == 0 {
			VidFinish()
			return
		}
		if int8(next) < vidAudioHead {
			if v.AudioState == 3 {
				v.FrameWaits = 1
			}
			return
		}
		// audio chunks after a frame are read at once
	}
}

func VidCloseFile() {
	v := &VidState
	v.AudioState = 0
	if v.file != nil {
		v.file.Close()
		v.file = nil
	}
}

// VidRefill moves the rest of the buffer from p to its start and fills it up from the file;
// the new position is 0.
func VidRefill(p int32) int32 {
	v := &VidState
	if p == 0 {
		return p
	}
	if p > vidBuffer {
		p = vidBuffer
	}
	left := vidBuffer - p
	v.FilePos += p
	copy(v.buf, v.buf[p:])
	if v.file != nil {
		io.ReadFull(v.file, v.buf[left:])
	}
	return 0
}

func VidFill() {
	v := &VidState
	v.ReadPtr = 0
	if v.file != nil {
		io.ReadFull(v.file, v.buf)
	}
}

func VidScheduleFrame(delay uint16) {
	v := &VidState
	v.FrameTicks += int16(delay + VidHeaderData.FrameDelay)
	if v.AudioState != 1 {
		return
	}
	v.AudioState++
	if v.AudioTicks != 0 {
		v.AudioWaiting = 1
		return
	}
	v.AudioState++
	VidAudioStart()
}

// VidReadAudio copies an audio chunk into the next audio buffer (Q-VID-01: each pass copies
// to the buffer's start).
func VidReadAudio(p int32) {
	v := &VidState
	v.AudioSize = v.word(p)
	p += 2
	v.AudioLeft = v.AudioSize
	for {
		n := uint32(vidBuffer - p)
		if n <= uint32(v.AudioSize) {
			p = VidRefill(p)
			n = vidAudioBuffer
		}
		n = min(n, uint32(v.AudioLeft))
		v.AudioLeft -= uint16(n)
		dst := v.AudioBufA
		if v.AudioBuffer == 0 {
			v.AudioLenA = v.AudioSize
		} else {
			dst = v.AudioBufB
			v.AudioLenB = v.AudioSize
		}
		for k := uint32(0); k < n && int(k) < len(dst); k++ {
			dst[k] = v.at(p + int32(k))
		}
		p += int32(n)
		if v.AudioLeft == 0 {
			break
		}
	}
	v.AudioBuffer = (v.AudioBuffer + 1) & 1
	v.ReadPtr = p
}

func VidTimerCallback() {
	v := &VidState
	v.FrameTicks--
	if v.AudioWaiting != 0 {
		v.AudioTicks--
		if v.AudioTicks == 0 {
			v.AudioWaiting = 0
		}
	}
}

func (v *VidPlayer) playing() ([]byte, uint16) {
	if v.AudioBuffer == 0 {
		return v.AudioBufB, v.AudioLenB
	}
	return v.AudioBufA, v.AudioLenA
}

// vidAudioDone is the sample's end: the next buffer is queued, unless the last was not
// taken yet.
func vidAudioDone(s *Sample) {
	v := &VidState
	if v.SampleDone == 1 {
		v.SampleRestart = 1
		return
	}
	buf, n := v.playing()
	s.Data = buf[:min(int(n), len(buf))]
	v.SampleDone = 1
}

func VidAudioStop() {
	v := &VidState
	if v.AudioState == 0 {
		return
	}
	v.AudioState = 0
	SoundStop(v.sampleHandle)
}

func VidAudioStart() {
	v := &VidState
	buf, n := v.playing()
	vidSample = Sample{
		Data:   buf[:min(int(n), len(buf))],
		Rate:   int(v.SampleRate),
		Volume: 0x7F,
		Pan:    0x8000,
		Done:   vidAudioDone,
	}
	v.sampleHandle = SoundStart(&vidSample)
}

// VidFrames decodes the movie at path (a DOS path) without timing or sound, calling fn with
// the screen and the palette (6-bit) after each frame: for a still from it (goplay's cover).
func VidFrames(path string, fn func(screen []byte, pal *[0x300]byte)) bool {
	if Width == 0 {
		Width, Height, ScreenSize = 320, 200, 64000
	}
	if len(Screen) < 64000 {
		Screen = make([]byte, ScreenSize)
	}
	BuildRowOffsets()
	SetClip(0, 0, Width, Height)
	v := &VidState
	if !dosfs.Exists(path) || !VidOpen(0, 0, path) {
		return false
	}
	for done := v.FramesDone; v.Done == 0; {
		if v.FramesDone != done {
			done = v.FramesDone
			fn(Screen[:64000], &v.Palette)
		}
		v.FrameWaits = 0
		VidDecodeChunk()
	}
	return true
}
