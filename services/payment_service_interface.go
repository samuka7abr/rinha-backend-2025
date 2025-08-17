package services

import (
	"context"

	"myRinhaGo/models"
)

type PaymentService interface {
	Add(ctx context.Context, p models.Payment) bool
	Summary(fromSec, toSec int64) (sum int64, count int64)
	Purge(ctx context.Context)
}
