// Package economy определяет общий контракт покупки: отдельного AI-кошелька нет.
package economy

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalid     = errors.New("bad_purchase")
	ErrNoCredits   = errors.New("no_credits")
	ErrConflict    = errors.New("purchase_conflict")
	ErrUnavailable = errors.New("purchase_unavailable")
)

type Cargo struct {
	Hold     map[string]float64
	Revision int64
}
type Request struct {
	Player, Key, Resource       string
	Amount, UnitPrice, Capacity float64
	Before                      Cargo
}
type Result struct {
	Known, Found   bool
	Cargo          *Cargo
	Credits, Spent int64
}
type Store interface {
	Purchase(context.Context, Request) (Result, error)
	LookupPurchase(context.Context, string, string, string, float64) (Result, error)
	PurchaseCargo(context.Context, string) (*Cargo, error)
}

func ValidKey(s string) bool {
	return len(s) > 0 && len(s) <= 128 && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}
func Intent(resource string, amount float64) ([32]byte, error) {
	if resource == "" || len(resource) > 24 || !utf8.ValidString(resource) || strings.ContainsRune(resource, 0) || !finite(amount) || amount <= 0 || amount > 1000 {
		return [32]byte{}, ErrInvalid
	}
	b, _ := json.Marshal(struct {
		Resource string
		Amount   float64
	}{resource, amount})
	return sha256.Sum256(b), nil
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func CopyHold(in map[string]float64) map[string]float64 {
	out := map[string]float64{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
func Clone(c *Cargo) *Cargo {
	if c == nil {
		return nil
	}
	return &Cargo{CopyHold(c.Hold), c.Revision}
}
func ValidateCargo(c Cargo) error {
	if c.Revision < 0 {
		return ErrInvalid
	}
	for k, v := range c.Hold {
		if k == "" || !finite(v) || v < 0 {
			return ErrInvalid
		}
	}
	return nil
}
func Prepare(r Request) ([32]byte, int64, Cargo, error) {
	hash, err := Intent(r.Resource, r.Amount)
	if err != nil {
		return hash, 0, Cargo{}, err
	}
	if !ValidKey(r.Key) || r.Player == "" || !finite(r.UnitPrice) || r.UnitPrice <= 0 || !finite(r.Capacity) || r.Capacity <= 0 || ValidateCargo(r.Before) != nil || r.Before.Revision == math.MaxInt64 {
		return hash, 0, Cargo{}, ErrInvalid
	}
	used := 0.0
	for _, v := range r.Before.Hold {
		used += v
	}
	if !finite(used) || used+r.Amount > r.Capacity+1e-8 {
		return hash, 0, Cargo{}, ErrInvalid
	}
	cost := math.Ceil(r.UnitPrice * r.Amount)
	if !finite(cost) || cost < 1 || cost >= float64(math.MaxInt64) {
		return hash, 0, Cargo{}, ErrInvalid
	}
	after := Cargo{CopyHold(r.Before.Hold), r.Before.Revision + 1}
	after.Hold[r.Resource] += r.Amount
	if !finite(after.Hold[r.Resource]) || after.Hold[r.Resource] <= r.Before.Hold[r.Resource] {
		return hash, 0, Cargo{}, ErrInvalid
	}
	return hash, int64(cost), after, nil
}
