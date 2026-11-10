package quest

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/maillunex-lang/CosmocRPG/server/internal/economy"
	"github.com/maillunex-lang/CosmocRPG/server/internal/world"
)

func TestPurchaseAtomicReplayAndStaleAutosave(t *testing.T) {
	ctx, st, pid := pgSetup(t)
	players := world.NewPGStore(st.pool)
	rec, err := players.Load(ctx, pid)
	if err != nil {
		t.Fatal(err)
	}
	rec.Hold = map[string]float64{"iron_ore": 2}
	if err = players.Save(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if _, _, err = st.GrantCredits(ctx, pid, "fund", 1000); err != nil {
		t.Fatal(err)
	}
	request := economy.Request{Player: pid, Key: "buy-1", Resource: "gas", Amount: 2, UnitPrice: 10.5, Capacity: 100, Before: economy.Cargo{Hold: rec.Hold, Revision: 0}}
	result, err := st.Purchase(ctx, request)
	if err != nil || !result.Found || result.Credits != 979 || result.Spent != 21 || result.Cargo.Revision != 1 || result.Cargo.Hold["gas"] != 2 {
		t.Fatal(result, err)
	}
	if err = players.Save(ctx, rec); err != nil {
		t.Fatal(err)
	} // снимок сделан до commit
	restored, err := players.Load(ctx, pid)
	if err != nil || restored.Hold["gas"] != 2 || restored.CargoRevision != 1 {
		t.Fatal("stale save overwrote purchase", restored, err)
	}
	request.UnitPrice = 100 // цена уже изменилась: replay не создаёт новой покупки
	result, err = st.Purchase(ctx, request)
	if err != nil || result.Credits != 979 || result.Spent != 21 || result.Cargo.Hold["gas"] != 2 {
		t.Fatal(result, err)
	}
	request.Resource = "scrap"
	if _, err = st.Purchase(ctx, request); !errors.Is(err, economy.ErrConflict) {
		t.Fatal("operation payload changed", err)
	}
	restored.Hold["iron_ore"] += 3
	restored.CargoRevision++
	if err = players.Save(ctx, restored); err != nil {
		t.Fatal(err)
	}
	result, err = st.LookupPurchase(ctx, pid, "buy-1", "gas", 2)
	if err != nil || result.Cargo.Revision != 2 || result.Cargo.Hold["iron_ore"] != 5 || result.Credits != 979 {
		t.Fatal("replay lost later cargo", result, err)
	}
}
func TestPurchaseRollbackAndNoFunds(t *testing.T) {
	ctx, st, pid := pgSetup(t)
	r := economy.Request{Player: pid, Key: "failure", Resource: "gas", Amount: 2, UnitPrice: 10, Capacity: 100, Before: economy.Cargo{Hold: map[string]float64{}}}
	if _, err := st.Purchase(ctx, r); !errors.Is(err, economy.ErrNoCredits) {
		t.Fatal(err)
	}
	if _, _, err := st.GrantCredits(ctx, pid, "fund", 1000); err != nil {
		t.Fatal(err)
	}
	// Ошибка после UPDATE кошелька/груза, на INSERT receipt: всё обязано откатиться.
	_, err := st.pool.Exec(ctx, `CREATE FUNCTION reject_purchase_receipt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected receipt failure'; END $$;
 CREATE TRIGGER reject_purchase BEFORE INSERT ON resource_purchase_receipts FOR EACH ROW EXECUTE FUNCTION reject_purchase_receipt()`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.Purchase(ctx, r); err == nil {
		t.Fatal("storage failure ignored")
	}
	state, err := st.LookupPurchase(ctx, pid, r.Key, r.Resource, r.Amount)
	if err != nil || state.Found || state.Credits != 1000 || state.Cargo.Revision != 0 || len(state.Cargo.Hold) != 0 {
		t.Fatal("partial purchase committed", state, err)
	}
}
func TestConcurrentPurchaseSingleCharge(t *testing.T) {
	ctx, st, pid := pgSetup(t)
	if _, _, err := st.GrantCredits(ctx, pid, "fund", 1000); err != nil {
		t.Fatal(err)
	}
	r := economy.Request{Player: pid, Key: "shared", Resource: "gas", Amount: 2, UnitPrice: 10, Capacity: 100, Before: economy.Cargo{Hold: map[string]float64{}}}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := st.Purchase(ctx, r)
			if err != nil || !res.Found {
				t.Error(res, err)
			}
		}()
	}
	wg.Wait()
	state, err := st.LookupPurchase(ctx, pid, r.Key, r.Resource, r.Amount)
	if err != nil || state.Credits != 980 || state.Cargo.Revision != 1 || state.Cargo.Hold["gas"] != 2 {
		t.Fatal(state, err)
	}
	r.Key = "stale-other-worker"
	if _, err = st.Purchase(ctx, r); !errors.Is(err, economy.ErrConflict) {
		t.Fatal("stale revision accepted", err)
	}
}
func TestMemoryPurchaseSameWalletAndReplay(t *testing.T) {
	ctx := context.Background()
	st := NewMemStore()
	if _, _, err := st.GrantCredits(ctx, "p", "fund", 100); err != nil {
		t.Fatal(err)
	}
	req := economy.Request{Player: "p", Key: "one", Resource: "gas", Amount: 1, UnitPrice: 10.5, Capacity: 100, Before: economy.Cargo{Hold: map[string]float64{}}}
	for i := 0; i < 2; i++ {
		r, err := st.Purchase(ctx, req)
		if err != nil || r.Credits != 89 || r.Cargo.Hold["gas"] != 1 {
			t.Fatal(r, err)
		}
	}
	data, err := st.Load(ctx, "p")
	if err != nil || data.Credits != 89 {
		t.Fatal("duplicated wallet", data, err)
	}
}
