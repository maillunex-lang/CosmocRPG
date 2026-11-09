package equipment

import (
	"context"
	"github.com/maillunex-lang/CosmocRPG/server/internal/fitting"
)

// GeneralStore mutates one owner's real inventory/profile atomically. Callback
// inputs are trusted server state; client transport must use typed commands.
type GeneralStore interface {
	Ensure(context.Context, string, string, string) (fitting.State, error)
	Load(context.Context, string, string) (fitting.State, error)
	Update(context.Context, string, string, int64, int64, func(*fitting.State) error) (fitting.State, error)
	GrantSpecializationXP(context.Context, string, string, string, string, int64) (bool, fitting.State, error)
}
