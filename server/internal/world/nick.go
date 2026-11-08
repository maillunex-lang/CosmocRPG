package world

import (
	"errors"
	"strings"
	"unicode"
)

// Ошибки ника.
var (
	ErrBadNick      = errors.New("bad_nick")
	ErrNickReserved = errors.New("nick_reserved")
	ErrNickTaken    = errors.New("nick_taken")
	ErrCharacterSet = errors.New("character_exists")
	ErrBadGender    = errors.New("bad_gender")
	ErrBadPilotSpec = errors.New("bad_pilot_spec")
)

// Подстроки, недопустимые в нике: служебные роли и грубые слова (список расширяется; проверка без учёта регистра и разделителей).
var reservedNick = []string{"admin", "админ", "moder", "модер", "master", "мастер", "support", "саппорт", "system", "систем", "server", "сервер", "gm_", "бот-",
	"fuck", "shit", "bitch", "nigg", "хуй", "хуе", "пизд", "бляд", "блят", "ебан", "ебат", "сука", "сучк", "гандон", "мудак", "пидор", "пидар", "залуп"}

// CleanNick проверяет и нормализует ник: 3-16 символов, латиница, кириллица, цифры, «_» и «-» внутри,
// хотя бы одна буква, не из одних цифр. Возвращает ник в исходном регистре.
func CleanNick(s string) (string, error) {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) < 3 || len(r) > 16 {
		return "", ErrBadNick
	}
	letters := 0
	for i, c := range r {
		isLetter := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 'а' && c <= 'я' || c >= 'А' && c <= 'Я' || c == 'ё' || c == 'Ё'
		switch {
		case isLetter:
			letters++
		case unicode.IsDigit(c) && c < 128:
		case (c == '_' || c == '-') && i > 0 && i < len(r)-1:
		default:
			return "", ErrBadNick
		}
	}
	if letters == 0 {
		return "", ErrBadNick
	}
	flat := strings.ToLower(strings.NewReplacer("_", "", "-", "", "ё", "е").Replace(s))
	for _, bad := range reservedNick {
		if strings.Contains(flat, strings.NewReplacer("_", "", "-", "", "ё", "е").Replace(bad)) {
			return "", ErrNickReserved
		}
	}
	return s, nil
}

// ValidGender: "m" или "f".
func ValidGender(g string) bool { return g == "m" || g == "f" }
