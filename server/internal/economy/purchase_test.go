package economy

import (
	"math"
	"testing"
)

func TestQuoteRejectsUnrepresentableAndInvalidPurchases(t *testing.T) {
	valid := Request{Player: "p", Key: "key", Resource: "gas", Amount: 2, UnitPrice: 10.5, Capacity: 100, Before: Cargo{Hold: map[string]float64{"iron_ore": 3}}}
	_, cost, after, err := Prepare(valid)
	if err != nil || cost != 21 || after.Revision != 1 || after.Hold["gas"] != 2 {
		t.Fatal(cost, after, err)
	}
	after.Hold["iron_ore"] = 99
	if valid.Before.Hold["iron_ore"] != 3 {
		t.Fatal("quote aliases original cargo")
	}
	for _, change := range []func(*Request){
		func(r *Request) { r.Amount = math.NaN() }, func(r *Request) { r.Amount = math.Inf(1) }, func(r *Request) { r.Amount = -1 },
		func(r *Request) { r.UnitPrice = math.Inf(1) }, func(r *Request) { r.UnitPrice = float64(math.MaxInt64) }, func(r *Request) { r.Capacity = 1 },
		func(r *Request) { r.Before.Revision = math.MaxInt64 }, func(r *Request) { r.Key = "" },
		func(r *Request) { r.Amount = 1e-30; r.Before.Hold = map[string]float64{"gas": 1}; r.UnitPrice = 1 },
	} {
		r := valid
		change(&r)
		if _, _, _, err := Prepare(r); err == nil {
			t.Fatalf("invalid quote accepted: %+v", r)
		}
	}
}
