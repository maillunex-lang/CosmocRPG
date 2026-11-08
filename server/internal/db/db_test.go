package db

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
)

// testPool даёт пул с собственной временной схемой (см. пакет testdb).
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return testdb.Pool(t)
}

func TestConnectFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := Connect(ctx, "postgres://nobody:x@127.0.0.1:1/none?sslmode=disable"); err == nil {
		t.Fatal("подключение к недоступной базе должно давать ошибку")
	}
	if _, err := Connect(ctx, "это не url"); err == nil {
		t.Fatal("некорректный адрес должен давать ошибку")
	}
}

func TestMigrateAppliesOnceAndIsIdempotent(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	first, err := Migrate(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 {
		t.Fatal("ни одна миграция не применена")
	}
	second, err := Migrate(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Fatalf("повторный запуск применил миграции: %v", second)
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&n); err != nil || n != len(first) {
		t.Fatalf("schema_migrations: n=%d err=%v", n, err)
	}
}

func TestRecordBootPersistsAndCounts(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	for want := int64(1); want <= 3; want++ {
		got, err := RecordBoot(ctx, pool, "test")
		if err != nil || got != want {
			t.Fatalf("запуск %d: got=%d err=%v", want, got, err)
		}
	}
}

func TestServerBootsRejectsEmptyVersion(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	if _, err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordBoot(ctx, pool, ""); err == nil {
		t.Fatal("ограничение CHECK должно отвергнуть пустую версию")
	}
}

func TestLoadMigrationsValidation(t *testing.T) {
	ok := fstest.MapFS{
		"migrations/0002_b.sql": {Data: []byte("select 2")},
		"migrations/0001_a.sql": {Data: []byte("select 1")},
	}
	got, err := loadMigrations(ok)
	if err != nil || len(got) != 2 || got[0].version != 1 || got[1].version != 2 {
		t.Fatalf("порядок миграций: %+v err=%v", got, err)
	}
	dup := fstest.MapFS{
		"migrations/0001_a.sql": {Data: []byte("x")},
		"migrations/0001_b.sql": {Data: []byte("y")},
	}
	if _, err := loadMigrations(dup); err == nil {
		t.Fatal("дубликат номера должен давать ошибку")
	}
	bad := fstest.MapFS{"migrations/abc_x.sql": {Data: []byte("x")}}
	if _, err := loadMigrations(bad); err == nil {
		t.Fatal("нечисловой номер должен давать ошибку")
	}
}

func TestMigrationFailureRollsBack(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	// Создаём таблицу заранее: миграция 0001 упадёт на CREATE TABLE.
	if _, err := pool.Exec(ctx, "CREATE TABLE server_boots (x int)"); err != nil {
		t.Fatal(err)
	}
	if _, err := Migrate(ctx, pool); err == nil {
		t.Fatal("ожидалась ошибка миграции")
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&n); err != nil || n != 0 {
		t.Fatalf("неудачная миграция записана как применённая: n=%d err=%v", n, err)
	}
}
