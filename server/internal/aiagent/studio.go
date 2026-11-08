package aiagent

import (
	"context"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aistudio"
)

// StudioSource отдаёт профили капитанов действующей версии AI Studio.
type StudioSource struct{ Svc *aistudio.Service }

// Profiles читает активную версию (кэширует вызывающий менеджер).
func (s StudioSource) Profiles(ctx context.Context) ([]Profile, error) {
	cfg, _, err := s.Svc.Active(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Profile, 0, len(cfg.Captains))
	for id, c := range cfg.Captains {
		if c.Name == "" {
			continue
		}
		out = append(out, Profile{ID: id, Name: c.Name, Profession: c.Profession})
	}
	return out, nil
}
