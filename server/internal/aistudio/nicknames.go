package aistudio

import (
	"math/rand"
	"strings"
)

// Nicknames возвращает детерминированно перемешанный список игровых ников: русские и английские вперемешку
// (примерно 40% кириллицей). Повторов нет (без учёта регистра).
func Nicknames(seed int64) []string {
	r := rand.New(rand.NewSource(seed*31 + 7))
	en := append([]string{}, nickEnglish...)
	ru := append([]string{}, nickRussian...)
	r.Shuffle(len(en), func(i, j int) { en[i], en[j] = en[j], en[i] })
	r.Shuffle(len(ru), func(i, j int) { ru[i], ru[j] = ru[j], ru[i] })
	out := make([]string, 0, len(en)+len(ru))
	seen := map[string]bool{}
	add := func(s string) {
		if k := strings.ToLower(s); !seen[k] {
			seen[k] = true
			out = append(out, s)
		}
	}
	for len(en) > 0 || len(ru) > 0 {
		takeRu := len(ru) > 0 && (len(en) == 0 || r.Intn(100) < 40)
		if takeRu {
			add(ru[0])
			ru = ru[1:]
		} else {
			add(en[0])
			en = en[1:]
		}
	}
	return out
}

// NickFor возвращает ник для капитана номер n (с 1); shift сдвигает выбор при коллизии с занятым именем.
func NickFor(seed int64, n, shift int) string {
	list := Nicknames(seed)
	return list[((n-1)+shift*97)%len(list)]
}
