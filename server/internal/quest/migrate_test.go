package quest

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maillunex-lang/CosmocRPG/server/internal/db"
)

func migrate(ctx context.Context, pool *pgxpool.Pool) ([]string, error) { return db.Migrate(ctx, pool) }
