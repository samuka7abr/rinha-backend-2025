package services

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// opcional: mantém o pool do PG “quente”
func StartHealthUpdater(ctx context.Context, pool *pgxpool.Pool) {
	t := time.NewTicker(30 * time.Second)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
				_ = pool.Ping(ctx2)
				cancel()
			}
		}
	}()
}
