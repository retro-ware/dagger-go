package meltysynth

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"
)

// readSamples decodes n 16-bit samples in slices, pausing between them: in a browser a
// pause is what returns to the event loop, so loading a large SoundFont on a goroutine
// doesn't freeze the game. (u4-go change; upstream used one binary.Read.)
func readSamples(r io.Reader, n int) ([]int16, error) {
	out := make([]int16, n)
	buf := make([]byte, 1<<20)
	for i := 0; i < n; {
		k := min(len(buf)/2, n-i)
		if _, err := io.ReadFull(r, buf[:2*k]); err != nil {
			return nil, err
		}
		for j := range k {
			out[i+j] = int16(binary.LittleEndian.Uint16(buf[2*j:]))
		}
		i += k
		time.Sleep(time.Millisecond)
	}
	return out, nil
}

type soundFontSampleData struct {
	bitsPerSample int32
	samples       []int16
}

func newSoundFontSampleData(r io.Reader) (*soundFontSampleData, error) {
	var n int
	chunkId, err := readFourCC(r)
	if err != nil {
		return nil, err
	}
	if chunkId != "LIST" {
		return nil, errors.New("the list chunk was not found")
	}

	var pos int32
	var end int32
	err = binary.Read(r, binary.LittleEndian, &end)
	if err != nil {
		return nil, err
	}

	listType, err := readFourCC(r)
	if err != nil {
		return nil, err
	}
	if listType != "sdta" {
		return nil, fmt.Errorf(`the type of the list chunk must be "sdta", but was %q`, listType)
	}
	pos += 4

	result := new(soundFontSampleData)

	for pos < end {
		var id string
		id, err = readFourCC(r)
		if err != nil {
			return nil, err
		}
		pos += 4

		var size int32
		err = binary.Read(r, binary.LittleEndian, &size)
		if err != nil {
			return nil, err
		}
		pos += 4

		switch id {
		case "smpl":
			result.bitsPerSample = 16
			result.samples, err = readSamples(r, int(size/2))
		case "sm24":
			// 24 bit audio is not supported.
			n, err = r.Read(make([]byte, size))
			if n != int(size) {
				return nil, errors.New("failed to read the 24 bit audio data")
			}
		default:
			return nil, fmt.Errorf("the info list contains an unknown id %q", id)
		}
		if err != nil {
			return nil, err
		}

		pos += size
	}

	if result.samples == nil {
		return nil, errors.New("no valid sample data was found")
	}

	return result, nil
}
