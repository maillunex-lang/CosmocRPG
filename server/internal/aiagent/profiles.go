package aiagent

import (
	"context"
	"hash/fnv"
)

// Profile — то, что агенту нужно знать о профиле AI-капитана из AI Studio.
type Profile struct {
	ID, Name, Profession string
}

// CaptainSource отдаёт профили капитанов действующей версии AI Studio.
type CaptainSource interface {
	Profiles(ctx context.Context) ([]Profile, error)
}

// professionBehavior — как профессия определяет поведение в мире. Неизвестные профессии держатся у станций.
var professionBehavior = map[string]string{
	"шахтёр-астероидщик": "miner", "сборщик утиля": "miner",
	"торговец": "trader", "пилот-грузоперевозчик": "trader", "логист": "trader", "квартирмейстер": "trader", "контрабандист": "trader",
	"разведчик": "scout", "исследователь": "scout", "картограф": "scout", "навигатор": "scout", "журналист": "scout", "учёный": "scout",
	"наёмник": "patrol", "страж сектора": "patrol", "охотник за головами": "patrol",
	"дипломат": "support", "бортмеханик": "support", "инженер-ремонтник": "support", "медик": "support",
}

// shipsFor — корабли каталога для поведения (выбор по зерну профиля).
var shipsFor = map[string][]string{
	"miner":   {"TMP-MINE"},
	"trader":  {"TMP-HAUL", "SUP-001"},
	"scout":   {"SCT-001"},
	"patrol":  {"FTR-001", "FTR-002", "COR-001"},
	"support": {"SUP-001", "COR-002"},
}

// BehaviorFor возвращает поведение по профессии.
func BehaviorFor(profession string) string {
	if b, ok := professionBehavior[profession]; ok {
		return b
	}
	return "support"
}

func seedOf(id string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return h.Sum32()
}

// ShipFor выбирает корабль для капитана: один и тот же капитан всегда летает на одном и том же корабле.
func ShipFor(behavior, id string) string {
	list := shipsFor[behavior]
	if len(list) == 0 {
		list = shipsFor["support"]
	}
	return list[int(seedOf(id))%len(list)]
}
