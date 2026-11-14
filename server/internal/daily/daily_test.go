package daily

import (
	"testing"
	"time"
)

func TestActivitiesAreDeterministicForDay(t *testing.T) {
	d := time.Date(2026, 11, 14, 12, 0, 0, 0, time.FixedZone("YEKT", 5*60*60))
	a, b := Activities(d), Activities(d.Add(8*time.Hour))
	if len(a) != 3 || len(b) != 3 || a[0].ID != b[0].ID { t.Fatalf("same UTC day must have same activities: %#v %#v", a, b) }
}

func TestReturnRewardHasWeeklyCap(t *testing.T) {
	if got := ReturnReward(1); got != 200 { t.Fatalf("streak 1: %d", got) }
	if got := ReturnReward(99); got != 500 { t.Fatalf("cap: %d", got) }
}
