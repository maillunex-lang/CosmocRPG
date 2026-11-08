// Package config читает конфигурацию сервера из переменных окружения.
package config

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Config — настройки сервера.
type Config struct {
	HTTPAddr        string        // COSMOC_HTTP_ADDR, по умолчанию ":8080"
	DatabaseURL     string        // COSMOC_DATABASE_URL, обязательна
	LogLevel        string        // COSMOC_LOG_LEVEL: debug|info|warn|error
	ShutdownTimeout time.Duration // COSMOC_SHUTDOWN_TIMEOUT, по умолчанию 10s
	WSMaxMessage    int64         // COSMOC_WS_MAX_MESSAGE, байт, по умолчанию 4096
	WSRatePerSec    int           // COSMOC_WS_RATE_PER_SEC, сообщений в секунду, по умолчанию 20

	FeatureLiving    bool   // COSMOC_FEATURE_LIVING: живые локации (враги, добыча, исследование), по умолчанию выключено
	AssetSignKeyFile string // COSMOC_ASSET_SIGN_KEY_FILE: приватный ключ подписи манифеста ресурсов (пусто: публикация ресурсов выключена)
	FeatureAI        bool   // COSMOC_FEATURE_AI: ИИ-игроки в мире (агент мира), по умолчанию выключено
	FeatureQuests    bool   // COSMOC_FEATURE_QUESTS: контракты и награды (флаг незавершённой функции), по умолчанию выключено

	AdminAddr         string        // COSMOC_ADMIN_ADDR, адрес админ-API (по умолчанию 127.0.0.1:8081); значение off выключает
	AdminCookieSecure bool          // COSMOC_ADMIN_COOKIE_SECURE, по умолчанию true (админка за HTTPS)
	AdminTrustProxy   bool          // COSMOC_ADMIN_TRUST_PROXY, доверять X-Forwarded-For от прокси, по умолчанию false
	AdminSessionTTL   time.Duration // COSMOC_ADMIN_SESSION_TTL, по умолчанию 12h
}

// Load собирает конфигурацию через функцию getenv (например, os.Getenv).
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		HTTPAddr:         def(getenv("COSMOC_HTTP_ADDR"), ":8080"),
		DatabaseURL:      getenv("COSMOC_DATABASE_URL"),
		AssetSignKeyFile: getenv("COSMOC_ASSET_SIGN_KEY_FILE"),
		LogLevel:         def(getenv("COSMOC_LOG_LEVEL"), "info"),
		ShutdownTimeout:  10 * time.Second,
		WSMaxMessage:     4096,
		WSRatePerSec:     20,

		AdminCookieSecure: true,
		AdminSessionTTL:   12 * time.Hour,
	}
	c.AdminAddr = def(getenv("COSMOC_ADMIN_ADDR"), "127.0.0.1:8081")
	if c.AdminAddr == "off" {
		c.AdminAddr = "" // админ-API выключен
	}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("COSMOC_DATABASE_URL не задана")
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("COSMOC_LOG_LEVEL: неизвестный уровень %q", c.LogLevel)
	}
	if v := getenv("COSMOC_SHUTDOWN_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("COSMOC_SHUTDOWN_TIMEOUT: некорректное значение %q", v)
		}
		c.ShutdownTimeout = d
	}
	if v := getenv("COSMOC_WS_MAX_MESSAGE"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return Config{}, fmt.Errorf("COSMOC_WS_MAX_MESSAGE: некорректное значение %q", v)
		}
		c.WSMaxMessage = n
	}
	if v := getenv("COSMOC_WS_RATE_PER_SEC"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return Config{}, fmt.Errorf("COSMOC_WS_RATE_PER_SEC: некорректное значение %q", v)
		}
		c.WSRatePerSec = n
	}
	for _, b := range []struct {
		name string
		dst  *bool
	}{{"COSMOC_ADMIN_COOKIE_SECURE", &c.AdminCookieSecure}, {"COSMOC_ADMIN_TRUST_PROXY", &c.AdminTrustProxy}, {"COSMOC_FEATURE_QUESTS", &c.FeatureQuests}, {"COSMOC_FEATURE_LIVING", &c.FeatureLiving}, {"COSMOC_FEATURE_AI", &c.FeatureAI}} {
		if v := getenv(b.name); v != "" {
			x, err := strconv.ParseBool(v)
			if err != nil {
				return Config{}, fmt.Errorf("%s: некорректное значение %q", b.name, v)
			}
			*b.dst = x
		}
	}
	if v := getenv("COSMOC_ADMIN_SESSION_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("COSMOC_ADMIN_SESSION_TTL: некорректное значение %q", v)
		}
		c.AdminSessionTTL = d
	}
	return c, nil
}

func def(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
