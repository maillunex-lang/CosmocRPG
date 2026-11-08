package chat

import (
	"regexp"
	"strings"
	"unicode"
)

// Простая фильтрация сообщений: ссылки и недопустимые слова. Это фильтр первой линии для альфы, а не гарантия: администратор
// дополняет список слов в настройках и может заглушить игрока.

var linkRe = regexp.MustCompile(`(?i)(https?://|www\.|ftp://|\b[\p{L}\d-]{2,}\.(ru|com|net|org|io|me|su|ly|gg|xyz|info|biz|рф)\b)`)

// HasLink сообщает, содержит ли текст ссылку или адрес сайта.
func HasLink(text string) bool { return linkRe.MatchString(text) }

// Основы запрещённых слов: проверяются как начало слова после нормализации (чтобы «небанальный» не срабатывал на «ебан»).
var ruStems = []string{"хуй", "хуе", "хуя", "хуи", "хую", "пизд", "ебан", "ебат", "ебал", "ебну", "ебуч", "ебис", "бляд", "блят", "блять", "мудак", "мудил", "мудозв", "гондон", "залуп", "пидор", "пидар", "педик", "шлюх", "дроч", "уебок", "уебан", "сука", "суки", "суку", "суке", "сучк", "сучар", "ебло", "выебы", "заеб", "наеб", "отъеб", "съеб"}
var enStems = []string{"fuck", "shit", "bitch", "cunt", "asshole", "nigg", "faggot", "whore", "dick", "pussy"}

// латинские двойники кириллических букв и цифры-замены
var lookalike = map[rune]rune{'a': 'а', 'c': 'с', 'e': 'е', 'o': 'о', 'p': 'р', 'x': 'х', 'y': 'у', 'k': 'к', 'm': 'м', 't': 'т', '0': 'о', '3': 'з', '4': 'ч', '6': 'б'}

func normalizeRu(tok string) string {
	var b strings.Builder
	var last rune
	for _, r := range strings.ToLower(tok) {
		if r == 'ё' {
			r = 'е'
		}
		if m, ok := lookalike[r]; ok {
			r = m
		}
		if r == last { // «хууууй» -> «хуй»
			continue
		}
		last = r
		b.WriteRune(r)
	}
	return b.String()
}

func tokens(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '*' })
}

// HasBadWord проверяет текст по встроенному списку и дополнительным словам (extra: точные слова или основы, без учёта регистра).
func HasBadWord(text string, extra []string) bool {
	// слитное написание через разделители («х.у.й», «х у й»): склеиваем одиночные буквы
	toks := tokens(text)
	var glued strings.Builder
	for _, t := range toks {
		if len([]rune(t)) == 1 {
			glued.WriteString(t)
		} else {
			if glued.Len() >= 3 {
				toks = append(toks, glued.String())
			}
			glued.Reset()
		}
	}
	if glued.Len() >= 3 {
		toks = append(toks, glued.String())
	}
	for _, t := range toks {
		ru := normalizeRu(strings.ReplaceAll(t, "*", ""))
		lat := strings.ToLower(t)
		for _, st := range ruStems {
			if strings.HasPrefix(ru, st) {
				return true
			}
		}
		for _, st := range enStems {
			if strings.HasPrefix(lat, st) {
				return true
			}
		}
		for _, w := range extra {
			w = strings.ToLower(strings.TrimSpace(w))
			if w != "" && (strings.HasPrefix(lat, w) || strings.HasPrefix(ru, normalizeRu(w))) {
				return true
			}
		}
	}
	return false
}

// CleanText убирает управляющие символы и лишние пробелы.
func CleanText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsControl(r) && r != ' ' {
			if r == '\n' || r == '\t' {
				b.WriteRune(' ')
			}
			continue
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
