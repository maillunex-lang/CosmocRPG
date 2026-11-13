package contract

import "testing"

func TestDeliveryValidation(t *testing.T) {
	valid := Delivery{ID: "c1", Resource: "iron_ore", Amount: 5, FromSystem: "aurora", ToSystem: "meridian", RewardCredits: 500, PenaltyCredits: 100}
	if err := valid.Validate(); err != nil { t.Fatal(err) }
	bad := valid; bad.Resource = "unknown"
	if err := bad.Validate(); err != ErrInvalidResource { t.Fatalf("want invalid resource, got %v", err) }
	bad = valid; bad.ToSystem = bad.FromSystem
	if err := bad.Validate(); err != ErrInvalidRoute { t.Fatalf("want invalid route, got %v", err) }
}
