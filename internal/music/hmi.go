package music

import (
	"encoding/binary"
	"errors"
	"sort"
)

// HMI ("HMI-MIDISONG061595"), the Human Machine Interfaces SOS driver's song format: MIDI
// tracks with a few changes. A note-on carries its length (a variable-length number) instead
// of a note-off, and 0xFE starts an SOS event. The song runs at (the word at 0xD4) ticks a
// second: a beat of 4 seconds at 4 times that many ticks a beat.

const (
	hmiDivisionOffset  = 0xD4
	hmiTrackCount      = 0xE4
	hmiTrackDirectory  = 0xE8
	hmiTrackData       = 0x57
	hmiTrackDesignates = 0x99
	hmiGeneralMIDI     = 0xA000
)

// Event is a MIDI channel message at a tick.
type Event struct {
	Tick   int
	Status byte
	D1, D2 byte
}

// Song is an HMI song: its events in tick order, its rate and its length (the last track's
// end).
type Song struct {
	Events      []Event
	TicksPerSec int
	Length      int
}

var errHMI = errors.New("hmi: not an HMI song")

// ParseHMI reads an HMI song's General MIDI tracks.
func ParseHMI(b []byte) (*Song, error) {
	if len(b) < hmiTrackDirectory+4 || string(b[:18]) != "HMI-MIDISONG061595" {
		return nil, errHMI
	}
	le16 := func(o int) int { return int(binary.LittleEndian.Uint16(b[o:])) }
	le32 := func(o int) int { return int(binary.LittleEndian.Uint32(b[o:])) }
	s := &Song{TicksPerSec: max(le16(hmiDivisionOffset), 1)}
	n, dir := le32(hmiTrackCount), le32(hmiTrackDirectory)
	if dir+n*4 > len(b) {
		return nil, errHMI
	}
	for i := range n {
		start := le32(dir + i*4)
		end := len(b)
		if i+1 < n {
			end = le32(dir + i*4 + 4)
		}
		if start+hmiTrackDesignates+32 > end || end > len(b) {
			continue
		}
		if !hmiForGeneralMIDI(b[start+hmiTrackDesignates:]) {
			continue
		}
		at := start + le32(start+hmiTrackData)
		if at < start || at > end {
			continue
		}
		length := parseHMITrack(b[at:end], &s.Events)
		s.Length = max(s.Length, length)
	}
	sort.SliceStable(s.Events, func(i, j int) bool {
		a, c := s.Events[i], s.Events[j]
		if a.Tick != c.Tick {
			return a.Tick < c.Tick
		}
		return a.Status&0xF0 == 0x80 && c.Status&0xF0 != 0x80 // note-offs first
	})
	return s, nil
}

// hmiForGeneralMIDI is whether a track's designations (eight device ids, 4 bytes apart; none
// set: every device) name the General MIDI device.
func hmiForGeneralMIDI(d []byte) bool {
	any := false
	for i := range 8 {
		switch binary.LittleEndian.Uint16(d[i*4:]) {
		case hmiGeneralMIDI:
			return true
		case 0, 0xFFFF:
		default:
			any = true
		}
	}
	return !any
}

// parseHMITrack appends a track's events; the tick of its end.
func parseHMITrack(d []byte, events *[]Event) int {
	p, tick := 0, 0
	var running byte
	vlq := func() int {
		v := 0
		for p < len(d) {
			c := d[p]
			p++
			v = v<<7 | int(c&127)
			if c < 128 {
				break
			}
		}
		return v
	}
	byteAt := func() byte {
		if p >= len(d) {
			p++
			return 0
		}
		p++
		return d[p-1]
	}
	for p < len(d) {
		tick += vlq()
		if p >= len(d) {
			break
		}
		status := d[p]
		if status&0x80 != 0 {
			p++
		} else {
			status = running
		}
		switch {
		case status < 0x80:
			return tick // data with no status: give up on the track
		case status < 0xF0:
			running = status
			switch status & 0xF0 {
			case 0xC0, 0xD0:
				*events = append(*events, Event{tick, status, byteAt(), 0})
			case 0x90:
				note, vel := byteAt(), byteAt()
				length := vlq()
				*events = append(*events, Event{tick, status, note, vel},
					Event{tick + length, 0x80 | status&15, note, 0})
			case 0xB0:
				c, v := byteAt(), byteAt()
				if c >= 102 && c <= 119 { // SOS's own controllers
					continue
				}
				*events = append(*events, Event{tick, status, c, v})
			default:
				a, c := byteAt(), byteAt()
				*events = append(*events, Event{tick, status, a, c})
			}
		case status == 0xFF:
			kind := byteAt()
			n := vlq()
			if kind == 0x2F {
				return tick
			}
			p += n
		case status == 0xFE:
			switch byteAt() {
			case 0x13, 0x15:
				p += 6
			case 0x12, 0x14:
				p += 2
			case 0x10:
				p += 2
				if p < len(d) {
					p += int(d[p]) + 5
				}
			default:
				return tick
			}
		case status == 0xF0 || status == 0xF7:
			p += vlq()
		default:
			return tick
		}
	}
	return tick
}
