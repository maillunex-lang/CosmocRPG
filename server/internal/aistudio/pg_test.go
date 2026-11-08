package aistudio

import (
	"context"
	"sync"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
)

func TestPGStoreCycleAndGameDataSafety(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	// игровые данные: игрок с кредитами
	if _, err := pool.Exec(ctx, `INSERT INTO players (token_hash, public_id, name, credits, pos_x, pos_y) VALUES (decode(repeat('ab',32),'hex'), 'aabbccdd11223344', 'Тест', 777, 3.5, -2.5)`); err != nil {
		t.Fatal(err)
	}
	snapshot := func() string {
		var s string
		_ = pool.QueryRow(ctx, `SELECT name || '|' || credits || '|' || pos_x || '|' || pos_y FROM players WHERE public_id = 'aabbccdd11223344'`).Scan(&s)
		return s
	}
	before := snapshot()

	s := NewService(NewPGStore(pool))
	_, v1, err := s.Generate(ctx, "owner", 20261108, 40)
	if err != nil || v1 == 0 {
		t.Fatalf("генерация: %v", err)
	}
	cfg, active, _ := s.Active(ctx)
	if active != v1 || len(cfg.Captains) != 40 {
		t.Fatal("активная версия не загрузилась из PostgreSQL")
	}
	// полный цикл: партия -> обучение -> предпросмотр -> публикация -> откат
	data, _, err := s.Export(ctx, "owner", ExportRequest{Kind: "batch", Size: 10, Index: 1})
	if err != nil {
		t.Fatal(err)
	}
	files := readPack(t, data)
	for n, b := range files {
		if len(n) > 9 && n[:9] == "captains/" {
			files[n] = []byte(string(b))
		}
	}
	rec, err := s.Preview(ctx, "owner", "t.zip", pack(t, files, func(m *Manifest) { m.PackID = "chatgpt-pg1" }), MergeOptions{})
	if err != nil || rec.Status != "validated" || rec.ID == 0 {
		t.Fatalf("предпросмотр: %v %s %+v", err, rec.Status, first2(rec.Report.Errors))
	}
	// пакет без изменений: всё unchanged, но публикация допустима
	v2, err := s.Publish(ctx, "owner", rec.ID, PublishOptions{AcceptWarnings: true})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.store.GetImport(ctx, rec.ID)
	if got.Status != "published" || got.VersionID == nil || *got.VersionID != v2 {
		t.Fatalf("импорт не помечен опубликованным: %+v", got.Status)
	}
	if len(got.Parsed) == 0 {
		t.Fatal("содержимое импорта не сохранено")
	}
	if _, err := s.Rollback(ctx, "owner", v1); err != nil {
		t.Fatal(err)
	}
	vs, _ := s.store.ListVersions(ctx, 10)
	activeCount := 0
	for _, v := range vs {
		if v.Active {
			activeCount++
		}
	}
	if len(vs) != 3 || activeCount != 1 || !vs[0].Active {
		t.Fatalf("версии: %+v", vs)
	}
	if after := snapshot(); after != before {
		t.Fatalf("игровые данные изменились: %q -> %q", before, after)
	}
	// сводка и тесты читаются из БД
	if run, err := s.RunTests(ctx, "owner", []string{"structure", "facts"}, 5); err != nil || run.ID == 0 || run.Passed != run.Total {
		t.Fatalf("прогон тестов: %v", err)
	}
	d, err := s.Dashboard(ctx)
	if err != nil || d.Captains != 40 || len(d.Imports) != 1 || len(d.Sessions) != 1 || d.LastTests == nil {
		t.Fatalf("сводка: %v %+v", err, d)
	}
}

func TestPGConcurrentPublishSingleActive(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	st := NewPGStore(pool)
	c := NewConfig()
	EnsureDefaults(&c)
	zero := int64(0)
	first, err := st.SaveVersion(ctx, NewVersion{Source: "generated", Config: c, ExpectActive: &zero})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok, conflicts := 0, 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			exp := first
			_, err := st.SaveVersion(ctx, NewVersion{Source: "edit", Config: c, ExpectActive: &exp})
			mu.Lock()
			defer mu.Unlock()
			switch err {
			case nil:
				ok++
			case ErrConflict:
				conflicts++
			default:
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if ok != 1 || conflicts != 9 {
		t.Fatalf("при гонке публикаций должна пройти ровно одна: прошло %d, конфликтов %d", ok, conflicts)
	}
	var active int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM ai_versions WHERE is_active`).Scan(&active)
	if active != 1 {
		t.Fatalf("активных версий %d", active)
	}
}
