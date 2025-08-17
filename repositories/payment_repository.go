package repositories

import (
	"context"
	"time"

	"myRinhaGo/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentWriter struct {
	pool      *pgxpool.Pool
	ch        chan models.Payment
	batchSize int
	batchMax  time.Duration
}

func NewPaymentWriter(pool *pgxpool.Pool, queueSize, batchSize int, batchMax time.Duration) *PaymentWriter {
	if queueSize < 1 {
		queueSize = 1000
	}
	if batchSize < 1 {
		batchSize = 1000
	}
	if batchMax <= 0 {
		batchMax = 50 * time.Millisecond
	}
	return &PaymentWriter{
		pool:      pool,
		ch:        make(chan models.Payment, queueSize),
		batchSize: batchSize,
		batchMax:  batchMax,
	}
}

func (w *PaymentWriter) Enqueue(p models.Payment) bool {
	select {
	case w.ch <- p:
		return true
	default:
		return false
	}
}

func (w *PaymentWriter) Run(ctx context.Context) {
	batch := make([]models.Payment, 0, w.batchSize)
	ticker := time.NewTicker(w.batchMax)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		_ = w.copyInsert(ctx, batch) // best-effort
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case p := <-w.ch:
			batch = append(batch, p)
			if len(batch) >= w.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (w *PaymentWriter) copyInsert(ctx context.Context, rows []models.Payment) error {
	src := pgx.CopyFromSlice(len(rows), func(i int) ([]interface{}, error) {
		return []interface{}{rows[i].Ts, rows[i].Amount}, nil
	})
	_, err := w.pool.CopyFrom(ctx, pgx.Identifier{"payments"}, []string{"ts", "amount"}, src)
	return err
}
