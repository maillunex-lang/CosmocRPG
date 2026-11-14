package ws

import "testing"

func TestDeliveryRewardUsesDistanceRiskAndResource(t *testing.T) {
	short := deliveryReward("iron_ore", "низкий", "aurora", "meridian", 2)
	long := deliveryReward("iron_ore", "высокий", "aurora", "redstorm", 2)
	crystals := deliveryReward("crystals", "средний", "aurora", "meridian", 2)
	if long <= short || crystals <= short || short <= 0 {
		t.Fatalf("rewards are not balanced: %d %d %d", short, long, crystals)
	}
}
