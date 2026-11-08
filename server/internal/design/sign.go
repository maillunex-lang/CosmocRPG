package design

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"
)

// Signer подписывает манифест ресурсов (RSA-SHA256, PKCS#1 v1.5, base64): так же, как манифесты обновлений, чтобы клиент проверял
// подпись тем же приёмом. Ключ ресурсов отдельный от ключа обновлений APK: утечка этого ключа не позволяет подменить само приложение.
type Signer struct{ key *rsa.PrivateKey }

// LoadSigner читает приватный ключ из PEM-файла (PKCS#8 или PKCS#1).
func LoadSigner(path string) (*Signer, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseSigner(raw)
}

// ParseSigner разбирает PEM-данные приватного ключа.
func ParseSigner(raw []byte) (*Signer, error) {
	blk, _ := pem.Decode(raw)
	if blk == nil {
		return nil, errors.New("ключ подписи ресурсов: не PEM")
	}
	if k, err := x509.ParsePKCS8PrivateKey(blk.Bytes); err == nil {
		if rk, ok := k.(*rsa.PrivateKey); ok {
			return checkSize(rk)
		}
		return nil, errors.New("ключ подписи ресурсов: нужен RSA")
	}
	rk, err := x509.ParsePKCS1PrivateKey(blk.Bytes)
	if err != nil {
		return nil, errors.New("ключ подписи ресурсов: не удалось разобрать")
	}
	return checkSize(rk)
}

func checkSize(k *rsa.PrivateKey) (*Signer, error) {
	if k.N.BitLen() < 2048 {
		return nil, errors.New("ключ подписи ресурсов: нужен RSA не короче 2048 бит")
	}
	return &Signer{key: k}, nil
}

// Sign возвращает подпись данных в base64.
func (s *Signer) Sign(data []byte) (string, error) {
	h := sha256.Sum256(data)
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// PublicPEM возвращает публичный ключ в PEM (для вшивания в клиент и проверок).
func (s *Signer) PublicPEM() string {
	der, _ := x509.MarshalPKIXPublicKey(&s.key.PublicKey)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// Verify проверяет подпись (для тестов и самопроверки при публикации).
func Verify(pub *rsa.PublicKey, data []byte, sigB64 string) bool {
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return false
	}
	h := sha256.Sum256(data)
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, h[:], sig) == nil
}
