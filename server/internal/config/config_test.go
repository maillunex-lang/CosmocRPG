package config

import (
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	c, err := Load(env(map[string]string{"COSMOC_DATABASE_URL": "postgres://x"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" || c.LogLevel != "info" || c.ShutdownTimeout != 10*time.Second ||
		c.WSMaxMessage != 4096 || c.WSRatePerSec != 20 ||
		c.AdminAddr != "127.0.0.1:8081" || !c.AdminCookieSecure || c.AdminTrustProxy || c.AdminSessionTTL != 12*time.Hour {
		t.Fatalf("неожиданные значения по умолчанию: %+v", c)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	if _, err := Load(env(nil)); err == nil {
		t.Fatal("ожидалась ошибка без COSMOC_DATABASE_URL")
	}
}

func TestLoadInvalidValues(t *testing.T) {
	base := map[string]string{"COSMOC_DATABASE_URL": "postgres://x"}
	cases := map[string]string{
		"COSMOC_LOG_LEVEL":        "loud",
		"COSMOC_SHUTDOWN_TIMEOUT": "abc",
		"COSMOC_WS_MAX_MESSAGE":   "-5",
		"COSMOC_WS_RATE_PER_SEC":  "0",

		"COSMOC_ADMIN_COOKIE_SECURE": "возможно",
		"COSMOC_ADMIN_TRUST_PROXY":   "2",
		"COSMOC_ADMIN_SESSION_TTL":   "-1h",
	}
	for k, v := range cases {
		m := map[string]string{}
		for bk, bv := range base {
			m[bk] = bv
		}
		m[k] = v
		if _, err := Load(env(m)); err == nil {
			t.Errorf("%s=%q: ожидалась ошибка", k, v)
		}
	}
}

func TestLoadOverrides(t *testing.T) {
	c, err := Load(env(map[string]string{
		"COSMOC_DATABASE_URL":     "postgres://x",
		"COSMOC_HTTP_ADDR":        ":9000",
		"COSMOC_LOG_LEVEL":        "debug",
		"COSMOC_SHUTDOWN_TIMEOUT": "3s",
		"COSMOC_WS_MAX_MESSAGE":   "1024",
		"COSMOC_WS_RATE_PER_SEC":  "5",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":9000" || c.LogLevel != "debug" || c.ShutdownTimeout != 3*time.Second ||
		c.WSMaxMessage != 1024 || c.WSRatePerSec != 5 {
		t.Fatalf("переопределения не применились: %+v", c)
	}
}

func TestAdminSettings(t *testing.T) {
	c, err := Load(env(map[string]string{
		"COSMOC_DATABASE_URL":        "postgres://x",
		"COSMOC_ADMIN_ADDR":          "0.0.0.0:9001",
		"COSMOC_ADMIN_COOKIE_SECURE": "false",
		"COSMOC_ADMIN_TRUST_PROXY":   "true",
		"COSMOC_ADMIN_SESSION_TTL":   "2h",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.AdminAddr != "0.0.0.0:9001" || c.AdminCookieSecure || !c.AdminTrustProxy || c.AdminSessionTTL != 2*time.Hour {
		t.Fatalf("настройки админки: %+v", c)
	}
	off, err := Load(env(map[string]string{"COSMOC_DATABASE_URL": "postgres://x", "COSMOC_ADMIN_ADDR": "off"}))
	if err != nil || off.AdminAddr != "" {
		t.Fatalf("off должен выключать админ-API: %+v err=%v", off, err)
	}
}

func TestCivilizationShadowRequiresAI(t *testing.T) {
	_, err := Load(env(map[string]string{"COSMOC_DATABASE_URL": "postgres://test", "COSMOC_AI_CIVILIZATION_SHADOW": "true"}))
	if err == nil {
		t.Fatal("shadow enabled without AI")
	}
	c, err := Load(env(map[string]string{"COSMOC_DATABASE_URL": "postgres://test", "COSMOC_AI_CIVILIZATION_SHADOW": "true", "COSMOC_FEATURE_AI": "true"}))
	if err != nil || !c.FeatureAICivilizationShadow {
		t.Fatal(c, err)
	}
}
