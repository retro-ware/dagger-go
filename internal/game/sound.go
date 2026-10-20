package game

import "github.com/retro-ware/dagger-go/internal/xn"

// sound.c, and the sound cache from objlib.c: four digital channels (three for one-shot
// sounds, the fourth for the ambient loop) on the xn sound driver. The original's SOS
// volumes run 0..0x7FFF; the driver takes 0..0x7F, so they are shifted down at the call.

const soundNoHandle = 0x12345678

// A SoundChannel is one of the four digital channels.
type SoundChannel struct {
	Handle   int32
	Priority int32
	Source   *Record
	Position [3]int32
	Volume   int32
	sample   xn.Sample
}

type soundCacheEntry struct {
	ID        int32
	Size      int32
	LastFrame int32
	Data      []byte
}

var musicTimerHandle int

var (
	SoundEnabled           bool
	SoundChannels          [4]SoundChannel
	SoundLastChannel       int32
	SoundLastID            int32
	SoundLastSize          int32
	SoundPanRange          int32
	SoundListenerYaw       int32
	AmbientRainChannel     int32
	AmbientCricketsChannel int32
	AmbientFireChannel     int32
	soundCache             [256]soundCacheEntry
	IsDaytime              uint8 // tamriel.c sets it
	soundCacheBytes        int32
)

// SoundInitMusic readies the sound channels and opens MIDI.BSA. The port plays its HMI
// songs through a General MIDI synth (internal/music) in place of the DOS drivers.
func SoundInitMusic() int32 {
	musicTimerHandle = xn.TimerRegister(140, xn.TimerTickCallback)
	for i := range SoundChannels {
		SoundChannels[i].Handle = soundNoHandle
	}
	MidiBSA = ArchiveOpen("midi.bsa", nil, 0)
	SoundEnabled = true
	return 1
}

func SoundShutdownMusic() {
	if !SoundEnabled {
		return
	}
	xn.TimerRemove(musicTimerHandle)
	for ch := int32(0); ch < 4; ch++ {
		SoundStopChannel(ch)
	}
}

func SoundApplyVolume() {}

func soundDone(h int32) bool { return xn.SoundDone(int(h)) }

func soundStart(ch int32, data []byte, volume, pan int32, loop bool) {
	c := &SoundChannels[ch]
	c.sample = xn.Sample{Data: data, Rate: 11025, Volume: int(volume >> 8), Pan: int(pan), Loop: loop}
	c.Handle = int32(xn.SoundStart(&c.sample))
}

func SoundChannelSetSource(o *Record, ch int32) {
	SoundChannels[ch].Source = o
	if o != nil {
		SoundChannels[ch].Position = [3]int32{o.X, o.Y, o.Z}
	}
}

// SoundVolumePan works out a sound's volume and pan from where it is (src) relative to the
// player; listener is set to src, as the original copied it.
func SoundVolumePan(listener *[3]int32, src [3]int32) (volume, pan int32) {
	*listener = src
	p := PlayerObject
	distance := xn.ApproxHypot(p.Y-src[1], xn.ApproxDist2D(p.X, p.Z, src[0], src[2]))
	if distance < 25 {
		return int32(int16(GameSettings.SoundVolume)) * 32767 / 128, 32768
	}
	if distance > SoundPanRange {
		volume = 0
	} else {
		volume = 32767 - distance*32767/SoundPanRange
	}
	angle := xn.AngleToPoint(p.X, p.Z, src[0], src[2])
	offset := AiAngleDiff(int32(p.Yaw), angle, &distance)
	if offset > 512 {
		offset = 512 - (offset - 512)
	}
	offset = offset << 15 / 512
	if CfgStereo != 0 {
		offset = -offset
	}
	if distance > 0 {
		pan = offset + 32768
	} else {
		pan = 32768 - offset
	}
	return volume * int32(int16(GameSettings.SoundVolume)) / 128, pan
}

// AiAngleDiff is the turn from angle to target (2048 to a circle); dir gets its sense.
func AiAngleDiff(angle, target int32, dir *int32) int32 {
	angle &= 2047
	target &= 2047
	diff := target - angle
	switch {
	case diff == 0:
		return 0
	case diff > 1024:
		*dir = -1
		return angle + (2048 - target)
	case diff > 0:
		*dir = 1
		return target - angle
	case diff < -1024:
		*dir = 1
		return target + (2048 - angle)
	}
	*dir = -1
	return angle - target
}

// soundFreeChannel finds a free one-shot channel, or stops one below priority.
func soundFreeChannel(priority int32, clear bool) int32 {
	ch := int32(0)
	for ; ch < 3; ch++ {
		if SoundChannels[ch].Handle == soundNoHandle {
			break
		}
		if soundDone(SoundChannels[ch].Handle) {
			if clear {
				SoundChannels[ch].Handle = soundNoHandle
			}
			break
		}
	}
	if ch == 3 {
		for ch = 0; ch < 3; ch++ {
			if SoundChannels[ch].Priority < priority {
				xn.SoundStop(int(SoundChannels[ch].Handle))
				break
			}
		}
	}
	return ch
}

// SoundPlaySample plays data from o (nil: where the channel last was). Priority -1 is the
// ambient loop on channel 3, -2 a loop on a one-shot channel.
func SoundPlaySample(data []byte, o *Record, priority int32) int32 {
	if !SoundEnabled || xn.SoundDigiDevice == -1 {
		return -1
	}
	loop := false
	var ch int32
	if priority == -1 {
		priority, loop, ch = 127, true, 3
		if SoundChannels[3].Handle != soundNoHandle {
			return -1
		}
	} else {
		if priority == -2 {
			priority, loop = 127, true
		}
		ch = soundFreeChannel(priority, true)
		if !loop && ch == 3 {
			return -1
		}
	}
	SoundLastChannel = ch
	c := &SoundChannels[ch]
	SoundChannelSetSource(o, ch)
	src := c.Position
	if c.Source != nil {
		src = [3]int32{c.Source.X, c.Source.Y, c.Source.Z}
	}
	volume, pan := SoundVolumePan(&c.Position, src)
	c.Priority = priority
	soundStart(ch, data, volume, pan, loop)
	return ch
}

// SoundPlaySampleFlat plays data at full volume, centred.
func SoundPlaySampleFlat(data []byte) int32 {
	if !SoundEnabled || xn.SoundDigiDevice == -1 {
		return -1
	}
	ch := soundFreeChannel(90, false)
	if ch == 3 {
		return -1
	}
	SoundChannels[ch].Priority = 90
	SoundChannels[ch].Source = nil
	soundStart(ch, data, 32767, 32768, false)
	return ch
}

// SoundPlaySampleLoud plays data looping at full volume.
func SoundPlaySampleLoud(data []byte) int32 {
	if !SoundEnabled || xn.SoundDigiDevice == -1 {
		return -1
	}
	ch := soundFreeChannel(127, false)
	if ch == 3 {
		return -1
	}
	SoundChannels[ch].Priority = 127
	SoundChannels[ch].Source = nil
	soundStart(ch, data, 32767, 32768, true)
	return ch
}

func SoundStopChannel(ch int32) {
	if SoundChannels[ch].Handle == soundNoHandle {
		return
	}
	xn.SoundStop(int(SoundChannels[ch].Handle))
	SoundChannels[ch].Handle = soundNoHandle
}

func SoundChannelDone(ch int32) bool {
	if SoundChannels[ch].Handle == soundNoHandle {
		return true
	}
	return soundDone(SoundChannels[ch].Handle)
}

func SoundPlay(id int32, o *Record, priority int32) int32 {
	if !SoundEnabled {
		return -1
	}
	return SoundPlaySample(SoundCacheLoad(id), o, priority)
}

func SoundPlayUI(id int32) int32 {
	if !SoundEnabled {
		return -1
	}
	return SoundPlaySampleFlat(SoundCacheLoad(id))
}

func SoundPlayAmbientLoop(id int32, o *Record, priority int32) int32 {
	if !SoundEnabled {
		return -1
	}
	return SoundPlaySample(SoundCacheLoad(id), o, -1)
}

func SoundPlayLoop(id int32, o *Record, priority int32) int32 {
	if !SoundEnabled {
		return -1
	}
	return SoundPlaySample(SoundCacheLoad(id), o, -2)
}

func SoundUpdateAmbient() {
	if PlayerEnvironment == 1 {
		if AmbientFireChannel != 0 {
			SoundStopChannel(AmbientFireChannel)
			AmbientFireChannel = 0
		}
		if uint8(ClimateWeathers[ClimateCategory()])&127 == 4 {
			if AmbientRainChannel == 0 {
				AmbientRainChannel = SoundPlayAmbientLoop(385, PlayerObject, 100)
			}
		} else if AmbientRainChannel != 0 {
			SoundStopChannel(AmbientRainChannel)
			AmbientRainChannel = 0
		}
		if IsDaytime == 0 && uint8(ClimateWeathers[ClimateCategory()]) < 2 {
			if AmbientCricketsChannel == 0 {
				AmbientCricketsChannel = SoundPlayAmbientLoop(375, PlayerObject, 100)
			}
		} else if AmbientCricketsChannel != 0 {
			SoundStopChannel(AmbientCricketsChannel)
			AmbientCricketsChannel = 0
		}
		return
	}
	if AmbientRainChannel != 0 {
		SoundStopChannel(AmbientRainChannel)
		AmbientRainChannel = 0
	}
	if AmbientCricketsChannel != 0 {
		SoundStopChannel(AmbientCricketsChannel)
		AmbientCricketsChannel = 0
	}
	if NearestFireDist < 300 && AmbientFireChannel == 0 && SoundChannels[3].Handle == soundNoHandle {
		AmbientFireChannel = SoundPlayAmbientLoop(242, NearestFire, 100)
		return
	}
	if AmbientFireChannel != 0 && NearestFireDist >= 300 {
		SoundStopChannel(AmbientFireChannel)
		AmbientFireChannel = 0
		return
	}
	SoundChannels[3].Source = NearestFire
}

func SoundStopAmbient() {
	for _, ch := range []*int32{&AmbientFireChannel, &AmbientCricketsChannel, &AmbientRainChannel} {
		if *ch != 0 {
			SoundStopChannel(*ch)
			*ch = 0
		}
	}
	SoundStopAll()
}

func SoundStopAll() {
	for ch := range SoundChannels {
		c := &SoundChannels[ch]
		if c.Handle == soundNoHandle {
			continue
		}
		if !soundDone(c.Handle) {
			xn.SoundStop(int(c.Handle))
		}
		c.Handle = soundNoHandle
	}
	RidingSound = -1
}

// SoundUpdateChannels follows the sounds' sources as the player moves.
func SoundUpdateChannels() {
	if !SoundEnabled {
		return
	}
	for i := int32(0); i < 4; i++ {
		SoundLastChannel = i
		c := &SoundChannels[i]
		if c.Handle == soundNoHandle {
			continue
		}
		if soundDone(c.Handle) {
			c.Handle = soundNoHandle
			continue
		}
		if c.Source == nil {
			continue
		}
		switch {
		case c.Source == NearestFire:
			SoundPanRange = 320
		case PlayerClass.Flags&1 != 0:
			SoundPanRange = 1024
		default:
			SoundPanRange = 768
		}
		volume, pan := SoundVolumePan(&c.Position, [3]int32{c.Source.X, c.Source.Y, c.Source.Z})
		c.Volume = volume
		if volume == 0 && i == 3 {
			SoundStopChannel(3)
			continue
		}
		xn.SoundSet(int(c.Handle), int(volume>>8), int(pan))
	}
	SoundListenerYaw = int32(PlayerObject.Yaw)
}

// SoundCacheLoad returns sound id from DAGGER.SND, through the cache.
func SoundCacheLoad(id int32) []byte {
	SoundLastID = id
	for i := range soundCache {
		if soundCache[i].ID == id && soundCache[i].Data != nil {
			SoundLastSize = soundCache[i].Size
			return soundCache[i].Data
		}
	}
	slot := 0
	for soundCache[slot].Data != nil {
		slot++
	}
	record := ArchiveFindRecord(DaggerSnd, "", id)
	size := ArchiveRecordSize(DaggerSnd, record)
	e := &soundCache[slot]
	e.LastFrame, e.ID, e.Size = FrameCounter, id, size
	e.Data = ArchiveReadRecord(DaggerSnd, record, make([]byte, size))
	soundCacheBytes += size
	soundCacheTrim()
	SoundLastSize = size
	return e.Data
}

func soundCacheTrim() {
	if soundCacheBytes < 393216 {
		return
	}
	for soundCacheBytes > 262144 {
		oldest, oldestFrame := -1, FrameCounter
		for i := range soundCache {
			if soundCache[i].Data != nil && oldestFrame > soundCache[i].LastFrame {
				oldestFrame, oldest = soundCache[i].LastFrame, i
			}
		}
		if oldest == -1 {
			return
		}
		soundCache[oldest].Data, soundCache[oldest].ID = nil, -1
		soundCacheBytes -= soundCache[oldest].Size
	}
}

func SoundCacheFreeAll() {
	for i := range soundCache {
		if soundCache[i].Data != nil {
			soundCache[i].Data, soundCache[i].ID = nil, -1
		}
	}
	soundCacheBytes = 0
}
