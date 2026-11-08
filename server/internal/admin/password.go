package admin

import "github.com/maillunex-lang/CosmocRPG/server/internal/passhash"

// Хеширование паролей вынесено в пакет passhash (общее с аккаунтами игроков); здесь тонкие обёртки.

// HashPassword возвращает хеш argon2id.
func HashPassword(password string) (string, error) { return passhash.HashPassword(password) }

// VerifyPassword проверяет пароль по хешу.
func VerifyPassword(password, encoded string) (bool, error) {
	return passhash.VerifyPassword(password, encoded)
}

var dummyHash = passhash.DummyHash

// MinPasswordLen — минимальная длина пароля администратора.
const MinPasswordLen = passhash.MinPasswordLen
