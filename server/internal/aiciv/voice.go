package aiciv

import "hash/fnv"

// VoiceStyle — устойчивая манера говорить. Один капитан не меняет её после перезапуска.
type VoiceStyle string

const (
	StyleShort VoiceStyle = "short"
	StyleWarm  VoiceStyle = "warm"
	StyleDry   VoiceStyle = "dry"
	StyleJoke  VoiceStyle = "joke"
	StyleTalk  VoiceStyle = "talk"
	StyleCare  VoiceStyle = "care"
)

func StyleOf(id string) VoiceStyle {
	h := fnv.New32a()
	_, _ = h.Write([]byte("voice:"))
	_, _ = h.Write([]byte(id))
	switch h.Sum32() % 6 {
	case 0:
		return StyleShort
	case 1:
		return StyleWarm
	case 2:
		return StyleDry
	case 3:
		return StyleJoke
	case 4:
		return StyleTalk
	default:
		return StyleCare
	}
}

func styleIndex(s VoiceStyle) int {
	switch s {
	case StyleShort:
		return 0
	case StyleWarm:
		return 1
	case StyleDry:
		return 2
	case StyleJoke:
		return 3
	case StyleTalk:
		return 4
	default:
		return 5
	}
}

func orderBy(lines []string, id string) []string {
	if len(lines) < 2 || id == "" {
		return lines
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return rotate(lines, int(h.Sum32()))
}
