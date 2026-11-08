// Package testdb — вспомогательный пакет для интеграционных тестов с PostgreSQL.
// Каждый вызов Pool даёт пул, привязанный к собственной временной схеме,
// поэтому тесты разных пакетов и разных функций не мешают друг другу
// и не затрагивают схему public.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EnvVar — переменная окружения с адресом тестовой базы.
const EnvVar = "COSMOC_TEST_DATABASE_URL"

// Pool создаёт изолированную схему и возвращает пул с search_path на неё.
// Если переменная окружения не задана, тест пропускается.
func Pool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv(EnvVar)
	if base == "" {
		t.Skip(EnvVar + " не задана: интеграционный тест пропущен")
	}
	ctx := context.Background()

	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	schema := "t_" + hex.EncodeToString(b)

	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}

	u, err := url.Parse(base)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()

	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})
	return pool
}
