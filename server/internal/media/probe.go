package media

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
)

const (
	MaxBytes    = 12 * 1024 * 1024
	MaxDuration = 10 * time.Minute
)

var (
	errNotAudio = errors.New("нужен файл MP3, OGG Vorbis или Opus")
	errTooLong  = errors.New("трек длиннее 10 минут")
	errBadAudio = errors.New("не удалось прочитать длительность: файл повреждён или это не звук")
)

// Probe проверяет сигнатуру, размер и длительность. Имя файла нужно только для расширения.
func Probe(name string, data []byte) (ext, mime, sum string, dur time.Duration, err error) {
	if len(data) == 0 || len(data) > MaxBytes {
		return "", "", "", 0, fmt.Errorf("размер файла должен быть от 1 байта до 12 МБ")
	}
	ext = strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
	switch ext {
	case "mp3", "ogg", "opus":
	default:
		return "", "", "", 0, errNotAudio
	}
	switch {
	case isMP3(data):
		if ext != "mp3" {
			return "", "", "", 0, errNotAudio
		}
		dur, err = mp3Duration(data)
		mime = "audio/mpeg"
	case isOgg(data):
		if ext != "ogg" && ext != "opus" {
			return "", "", "", 0, errNotAudio
		}
		dur, err = oggDuration(data)
		mime = "audio/ogg"
	default:
		return "", "", "", 0, errNotAudio
	}
	if err != nil {
		return "", "", "", 0, err
	}
	if dur <= 0 || dur > MaxDuration {
		if dur > MaxDuration {
			return "", "", "", 0, errTooLong
		}
		return "", "", "", 0, errBadAudio
	}
	h := sha256.Sum256(data)
	return ext, mime, hex.EncodeToString(h[:]), dur, nil
}

func isMP3(b []byte) bool {
	if len(b) >= 3 && (string(b[:3]) == "ID3" || string(b[:3]) == "TAG") {
		return true
	}
	_, _, _, ok := mp3Frame(b)
	return ok
}

func isOgg(b []byte) bool {
	return len(b) >= 4 && string(b[:4]) == "OggS"
}

func mp3Duration(b []byte) (time.Duration, error) {
	off := 0
	if len(b) >= 10 && string(b[:3]) == "ID3" {
		sz := int(b[6]&0x7f)<<21 | int(b[7]&0x7f)<<14 | int(b[8]&0x7f)<<7 | int(b[9]&0x7f)
		off = 10 + sz
		if b[5]&0x10 != 0 {
			off += 10
		}
		if off > len(b) {
			return 0, errBadAudio
		}
	}
	var samples float64
	frames := 0
	for off+4 <= len(b) {
		samp, rate, n, ok := mp3Frame(b[off:])
		if !ok {
			left := len(b) - off
			if frames > 0 && (left <= 256 || (left == 128 && string(b[off:off+3]) == "TAG")) {
				break
			}
			return 0, errBadAudio
		}
		if n <= 0 || off+n > len(b) {
			return 0, errBadAudio
		}
		samples += float64(samp) / float64(rate)
		frames++
		off += n
		if frames > 200000 {
			return 0, errBadAudio
		}
	}
	if frames == 0 {
		return 0, errBadAudio
	}
	return time.Duration(samples * float64(time.Second)), nil
}

func mp3Frame(b []byte) (samples, rate, length int, ok bool) {
	if len(b) < 4 || b[0] != 0xFF || b[1]&0xE0 != 0xE0 {
		return 0, 0, 0, false
	}
	ver := (b[1] >> 3) & 0x3
	layer := (b[1] >> 1) & 0x3
	brIdx := int(b[2] >> 4)
	srIdx := int((b[2] >> 2) & 0x3)
	pad := int((b[2] >> 1) & 0x1)
	if ver == 1 || layer != 1 || brIdx == 0 || brIdx == 15 || srIdx == 3 {
		return 0, 0, 0, false
	}
	var br int
	mpeg1 := ver == 3
	if mpeg1 {
		br = []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}[brIdx]
		rate = []int{44100, 48000, 32000}[srIdx]
		samples = 1152
		length = (144000 * br) / rate
		if pad == 1 {
			length++
		}
	} else {
		br = []int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0}[brIdx]
		if ver == 2 {
			rate = []int{22050, 24000, 16000}[srIdx]
		} else {
			rate = []int{11025, 12000, 8000}[srIdx]
		}
		samples = 576
		length = (72000 * br) / rate
		if pad == 1 {
			length++
		}
	}
	if length < 4 || rate == 0 {
		return 0, 0, 0, false
	}
	return samples, rate, length, true
}

func oggDuration(b []byte) (time.Duration, error) {
	off := 0
	var rate int
	var preSkip int
	var opus bool
	var lastGran int64 = -1
	pages := 0
	for off+27 <= len(b) {
		if string(b[off:off+4]) != "OggS" || b[off+4] != 0 {
			return 0, errBadAudio
		}
		gran := int64(le64(b[off+6 : off+14]))
		nseg := int(b[off+26])
		if off+27+nseg > len(b) {
			return 0, errBadAudio
		}
		payload := 0
		for i := 0; i < nseg; i++ {
			payload += int(b[off+27+i])
		}
		body := off + 27 + nseg
		if body+payload > len(b) {
			return 0, errBadAudio
		}
		if pages == 0 && payload > 0 {
			p := b[body : body+payload]
			switch {
			case len(p) >= 30 && p[0] == 1 && string(p[1:7]) == "vorbis":
				rate = int(le32(p[12:16]))
			case len(p) >= 16 && string(p[:8]) == "OpusHead":
				opus = true
				preSkip = int(le16(p[10:12]))
				rate = 48000
			default:
				return 0, errBadAudio
			}
		}
		if gran >= 0 {
			lastGran = gran
		}
		off = body + payload
		pages++
		if pages > 100000 {
			return 0, errBadAudio
		}
	}
	if rate <= 0 || lastGran < 0 {
		return 0, errBadAudio
	}
	samples := lastGran
	if opus {
		samples -= int64(preSkip)
	}
	if samples <= 0 {
		return 0, errBadAudio
	}
	return time.Duration(float64(samples) / float64(rate) * float64(time.Second)), nil
}

func le16(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 }
func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}
func le64(b []byte) uint64 {
	return uint64(le32(b[:4])) | uint64(le32(b[4:]))<<32
}
