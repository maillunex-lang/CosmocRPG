// Package admin — серверная часть админ-панели: вход, сессии, журнал действий.
package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Параметры argon2id (рекомендация OWASP: не менее 19 МиБ, t=2; берём с запасом).
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // КиБ
	argonThreads = 2
	argonKeyLen  = 32
	saltLen      = 16

	// MinPasswordLen — минимальная длина пароля администратора.
	MinPasswordLen = 12
)

// HashPassword возвращает хеш пароля в формате PHC: $argon2id$v=19$m=..,t=..,p=..$соль$хеш.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword сравнивает пароль с хешем за постоянное время.
func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	// ["", "argon2id", "v=19", "m=..,t=..,p=..", соль, хеш]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("неподдерживаемый формат хеша")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errors.New("неподдерживаемая версия argon2")
	}
	var memory, time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false, errors.New("некорректные параметры хеша")
	}
	// Защита от «вредных» параметров в испорченных данных.
	if memory > 1024*1024 || time > 20 || threads == 0 {
		return false, errors.New("параметры хеша вне допустимых пределов")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, errors.New("некорректный хеш")
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// dummyHash используется, чтобы вход несуществующего пользователя занимал столько же времени.
var dummyHash, _ = HashPassword("не-существующий-пользователь")
