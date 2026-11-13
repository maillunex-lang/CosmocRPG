package contract

import "errors"

var (
	ErrInvalidResource = errors.New("invalid contract resource")
	ErrInvalidAmount   = errors.New("invalid contract amount")
	ErrInvalidRoute    = errors.New("invalid contract route")
)

type Delivery struct {
	ID              string  `json:"id"`
	Resource        string  `json:"resource"`
	Amount          float64 `json:"amount"`
	FromSystem      string  `json:"from_system"`
	ToSystem        string  `json:"to_system"`
	RewardCredits   int64   `json:"reward_credits"`
	PenaltyCredits  int64   `json:"penalty_credits"`
	Risk            string  `json:"risk"`
	ExpiresAt       int64   `json:"expires_at"`
}

func (d Delivery) Validate() error {
	if d.Resource != "iron_ore" && d.Resource != "crystals" && d.Resource != "gas" && d.Resource != "scrap" {
		return ErrInvalidResource
	}
	if d.Amount <= 0 || d.Amount > 100000 {
		return ErrInvalidAmount
	}
	if d.FromSystem == "" || d.ToSystem == "" || d.FromSystem == d.ToSystem {
		return ErrInvalidRoute
	}
	if d.RewardCredits < 0 || d.PenaltyCredits < 0 {
		return ErrInvalidAmount
	}
	return nil
}
