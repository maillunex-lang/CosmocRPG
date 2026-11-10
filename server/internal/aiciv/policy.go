// Package aiciv содержит основу AI-PLAYERS-001; не заменяет gameplay/кошельки.
package aiciv

import "errors"

type Action string

const (
	Travel         Action = "travel"
	Buy            Action = "buy"
	Mine           Action = "mine"
	Sell           Action = "sell"
	Attack         Action = "attack"
	Repair         Action = "repair"
	Jump           Action = "jump"
	Quest          Action = "quest_interact"
	Manufacture    Action = "manufacture"
	Auction        Action = "auction"
	Party          Action = "contribute_operation"
	CorpLeadership Action = "corporate_leadership"
	PermanentDeath Action = "permanent_death"
	Enterprise     Action = "personal_enterprise"
	ExternalLLM    Action = "external_llm"
)

var ErrUnavailable = errors.New("capability_unavailable")
var ErrPolicy = errors.New("policy_denied")

// Capabilities содержит только действия с реально подключённым общим валидатором.
// Наличие файла доменной системы не является разрешением выполнения действия.
type Capabilities map[Action]bool

func (c Capabilities) Authorize(a Action) error {
	switch a {
	case CorpLeadership, PermanentDeath, Enterprise, ExternalLLM:
		return ErrPolicy
	case Travel, Buy, Mine, Sell, Attack, Repair, Jump, Quest, Manufacture, Auction, Party:
		if c[a] {
			return nil
		}
	}
	return ErrUnavailable
}
