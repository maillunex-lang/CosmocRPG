package media

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func mp3FrameBytes() []byte {
	// MPEG1 Layer3, 128 кбит/с, 44100 Гц, без паддинга: 417 байт.
	f := bytes.Repeat([]byte{0}, 417)
	copy(f, []byte{0xFF, 0xFB, 0x90, 0x00})
	return f
}

func TestProbeMP3(t *testing.T) {
	data := mp3FrameBytes()
	ext, mime, sum, dur, err := Probe("song.mp3", data)
	if err != nil {
		t.Fatal(err)
	}
	if ext != "mp3" || mime != "audio/mpeg" || len(sum) != 64 {
		t.Fatalf("ext=%s mime=%s sum=%s", ext, mime, sum)
	}
	samples := 1152.0 / 44100.0
	want := time.Duration(samples * float64(time.Second))
	if dur < want-time.Millisecond || dur > want+time.Millisecond {
		t.Fatalf("длительность %s, ожидали около %s", dur, want)
	}
}

func TestProbeRejectsNonAudioAndMismatch(t *testing.T) {
	if _, _, _, _, err := Probe("a.mp3", []byte("<html>нет</html>")); err == nil {
		t.Fatal("html принят")
	}
	if _, _, _, _, err := Probe("a.ogg", mp3FrameBytes()); err == nil {
		t.Fatal("mp3 под видом ogg принят")
	}
	if _, _, _, _, err := Probe("a.txt", mp3FrameBytes()); err == nil {
		t.Fatal("чужое расширение принято")
	}
	if _, _, _, _, err := Probe("a.mp3", nil); err == nil {
		t.Fatal("пустой файл принят")
	}
	big := bytes.Repeat([]byte{0}, MaxBytes+1)
	if _, _, _, _, err := Probe("a.mp3", big); err == nil || !strings.Contains(err.Error(), "12") {
		t.Fatalf("лимит размера: %v", err)
	}
}

func TestProbeRejectsOverTenMinutes(t *testing.T) {
	// MPEG2.5 Layer3, 8 кбит/с, 8000 Гц: кадр 72 байта, 576 сэмплов (0.072 с).
	frame := bytes.Repeat([]byte{0}, 72)
	copy(frame, []byte{0xFF, 0xE3, 0x18, 0x00})
	n := 8334 // > 10 минут
	data := bytes.Repeat(frame, n)
	_, _, _, _, err := Probe("long.mp3", data)
	if err == nil || !strings.Contains(err.Error(), "10") {
		t.Fatalf("ожидали отказ по длительности, получили %v", err)
	}
	short := bytes.Repeat(frame, 2)
	if _, _, _, _, err := Probe("short.mp3", short); err != nil {
		t.Fatal(err)
	}
}

func TestProbeOggVorbis(t *testing.T) {
	data := oggVorbisOneSecond()
	ext, mime, _, dur, err := Probe("theme.ogg", data)
	if err != nil {
		t.Fatal(err)
	}
	if ext != "ogg" || mime != "audio/ogg" {
		t.Fatalf("%s %s", ext, mime)
	}
	if dur < 900*time.Millisecond || dur > 1100*time.Millisecond {
		t.Fatalf("длительность %s", dur)
	}
}

func oggVorbisOneSecond() []byte {
	id := make([]byte, 30)
	id[0] = 1
	copy(id[1:], []byte("vorbis"))
	id[11] = 1                                        // channels
	id[12], id[13], id[14], id[15] = 0x44, 0xAC, 0, 0 // 44100
	audio := []byte{0x00, 0x01, 0x02, 0x03}
	return append(oggPage(0, 0x02, id), oggPage(44100, 0x04, audio)...)
}

func oggPage(gran int64, headerType byte, payload []byte) []byte {
	var page []byte
	page = append(page, 'O', 'g', 'g', 'S', 0, headerType)
	var g [8]byte
	for i := 0; i < 8; i++ {
		g[i] = byte(gran >> (8 * i))
	}
	page = append(page, g[:]...)
	page = append(page, 1, 0, 0, 0) // serial
	page = append(page, 0, 0, 0, 0) // seq
	page = append(page, 0, 0, 0, 0) // crc, не проверяем
	page = append(page, 1, byte(len(payload)))
	return append(page, payload...)
}
