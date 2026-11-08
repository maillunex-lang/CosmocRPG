package design

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
	"github.com/maillunex-lang/CosmocRPG/server/internal/testdb"
)

func asset(el, sha string) Asset {
	return Asset{ElementID: el, FileName: "f.svg", Mime: "image/svg+xml", SHA256: sha, Size: 3, Data: []byte("abc"), Report: newReport()}
}

func sha(c byte) string {
	b := make([]byte, 64)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

func runStoreContract(t *testing.T, st Store) {
	ctx := context.Background()
	a1, err := st.Add(ctx, asset("icon:gauge", sha('a')))
	if err != nil || a1.Version != 1 || a1.Status != StatusDraft {
		t.Fatalf("первая: %+v %v", a1, err)
	}
	a2, _ := st.Add(ctx, asset("icon:gauge", sha('b')))
	b1, _ := st.Add(ctx, asset("icon:coins", sha('c')))
	if a2.Version != 2 || b1.Version != 1 {
		t.Fatalf("версии считаются по элементу: %d %d", a2.Version, b1.Version)
	}
	if v, err := st.Publish(ctx, a1.ID); err != nil || v != 1 {
		t.Fatalf("публикация: %d %v", v, err)
	}
	if _, err := st.Publish(ctx, a1.ID); !errors.Is(err, ErrState) {
		t.Fatalf("повторная публикация: %v", err)
	}
	if v, err := st.Publish(ctx, a2.ID); err != nil || v != 2 {
		t.Fatalf("вторая публикация: %d %v", v, err)
	}
	pub, mv, _ := st.Published(ctx)
	if len(pub) != 1 || pub[0].ID != a2.ID || mv != 2 {
		t.Fatalf("опубликована одна — последняя: %+v (манифест %d)", pub, mv)
	}
	got, err := st.BySHA(ctx, sha('b'))
	if err != nil || string(got.Data) != "abc" {
		t.Fatalf("раздача по сумме: %+v %v", got, err)
	}
	if _, err := st.BySHA(ctx, sha('a')); !errors.Is(err, ErrNotFound) {
		t.Fatalf("архивная не раздаётся: %v", err)
	}
	if _, err := st.BySHA(ctx, sha('c')); !errors.Is(err, ErrNotFound) {
		t.Fatalf("черновик не раздаётся: %v", err)
	}
	if err := st.Delete(ctx, a2.ID); !errors.Is(err, ErrPublished) {
		t.Fatalf("опубликованную удалять нельзя: %v", err)
	}
	if err := st.Delete(ctx, a1.ID); err != nil {
		t.Fatalf("архивную можно: %v", err)
	}
	if _, err := st.Get(ctx, a1.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("удалённая: %v", err)
	}
	if v, err := st.Unpublish(ctx, "icon:gauge"); err != nil || v != 3 {
		t.Fatalf("снятие: %d %v", v, err)
	}
	if _, err := st.Unpublish(ctx, "icon:gauge"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("повторное снятие: %v", err)
	}
	list, _ := st.List(ctx, "icon:gauge")
	if len(list) != 1 || list[0].Status != StatusArchived || len(list[0].Data) != 0 {
		t.Fatalf("список не должен нести содержимое: %+v", list)
	}
}

func TestStoreContractMemory(t *testing.T) { runStoreContract(t, NewMemStore()) }

func TestStoreContractPostgres(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	runStoreContract(t, NewPGStore(pool))
}

func TestPGConcurrentUploadsGetDistinctVersions(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	st := NewPGStore(pool)
	var wg sync.WaitGroup
	versions := make(chan int, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a, err := st.Add(ctx, asset("icon:gauge", sha(byte('a'+i))))
			if err != nil {
				t.Error(err)
				return
			}
			versions <- a.Version
		}(i)
	}
	wg.Wait()
	close(versions)
	seen := map[int]bool{}
	for v := range versions {
		if seen[v] {
			t.Fatalf("версия %d выдана дважды", v)
		}
		seen[v] = true
	}
	if len(seen) != 12 {
		t.Fatalf("версий %d из 12", len(seen))
	}
}

func TestPGOnePublishedPerElement(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	st := NewPGStore(pool)
	a, _ := st.Add(ctx, asset("icon:gauge", sha('a')))
	b, _ := st.Add(ctx, asset("icon:gauge", sha('b')))
	var wg sync.WaitGroup
	for _, id := range []int64{a.ID, b.ID} {
		wg.Add(1)
		go func(id int64) { defer wg.Done(); _, _ = st.Publish(ctx, id) }(id)
	}
	wg.Wait()
	pub, _, _ := st.Published(ctx)
	if len(pub) != 1 {
		t.Fatalf("после параллельных публикаций должна остаться одна опубликованная замена, а их %d", len(pub))
	}
}
