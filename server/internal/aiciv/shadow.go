package aiciv

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/maillunex-lang/CosmocRPG/server/internal/aiagent"
)

// Shadow синхронизирует постоянные идентичности, ничего не спавнит и не исполняет.
// Отдельная зависимость от CaptainSource сохраняет совместимость ZIP Studio.
// Смена пакета не удаляет отсутствующих в нём капитанов и их память.
type Shadow struct {
	Source aiagent.CaptainSource
	Store  *Store
	Log    *slog.Logger
}

func (s *Shadow) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	var profiles []aiagent.Profile
	cursor := 0
	refresh := time.Time{}
	for {
		if ctx.Err() != nil {
			return
		}
		if time.Now().After(refresh) {
			request, cancel := context.WithTimeout(ctx, 3*time.Second)
			ps, err := s.Source.Profiles(request)
			cancel()
			if err != nil {
				s.Log.Error("AI shadow: профили недоступны", "err", err)
			} else {
				sort.Slice(ps, func(i, j int) bool { return ps[i].ID < ps[j].ID })
				profiles = ps
				if len(profiles) > 300 {
					profiles = profiles[:300]
				}
				if len(profiles) == 0 {
					cursor = 0
				} else {
					cursor %= len(profiles)
				}
			}
			refresh = time.Now().Add(time.Minute)
		}
		request, cancel := context.WithTimeout(ctx, 2*time.Second)
		processed := 0
		for processed < 16 && processed < len(profiles) && request.Err() == nil {
			p := profiles[cursor]
			cursor = (cursor + 1) % len(profiles)
			processed++
			if _, err := s.Store.Ensure(request, p.ID); err != nil {
				s.Log.Error("AI shadow: идентичность", "captain", p.ID, "err", err)
			}
		}
		if err := s.Store.Prune(request, time.Now()); err != nil {
			s.Log.Error("AI shadow: очистка памяти", "err", err)
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
