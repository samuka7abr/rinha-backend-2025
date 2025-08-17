package services

import (
	"context"
	"sync/atomic"
	"time"

	"myRinhaGo/models"
	"myRinhaGo/repositories"
)

type bucket struct {
	ts  int64
	sum int64
	cnt int64
}

type paymentService struct {
	buckets []bucket
	size    int64
	writer  *repositories.PaymentWriter
	redis   *RedisService
}

func NewPaymentService(sizeSeconds int64, writer *repositories.PaymentWriter, redis *RedisService) PaymentService {
	if sizeSeconds < 2 {
		sizeSeconds = 2
	}
	return &paymentService{
		buckets: make([]bucket, sizeSeconds),
		size:    sizeSeconds,
		writer:  writer,
		redis:   redis,
	}
}

func (s *paymentService) Add(ctx context.Context, p models.Payment) bool {
	sec := p.Ts
	if sec == 0 {
		sec = time.Now().Unix()
	}

	i := sec % s.size
	b := &s.buckets[i]
	if atomic.LoadInt64(&b.ts) != sec {
		atomic.StoreInt64(&b.ts, sec)
		atomic.StoreInt64(&b.sum, 0)
		atomic.StoreInt64(&b.cnt, 0)
	}
	atomic.AddInt64(&b.sum, p.Amount)
	atomic.AddInt64(&b.cnt, 1)

	if s.redis != nil {
		s.redis.Enqueue(sec, p.Amount)
	}

	if s.writer != nil {
		s.writer.Enqueue(models.Payment{Ts: sec, Amount: p.Amount})
	}
	return true
}

func (s *paymentService) Summary(fromSec, toSec int64) (sum int64, count int64) {
	if toSec < fromSec {
		return 0, 0
	}
	if toSec-fromSec+1 > s.size {
		fromSec = toSec - s.size + 1
	}
	for sec := fromSec; sec <= toSec; sec++ {
		b := &s.buckets[sec%s.size]
		if atomic.LoadInt64(&b.ts) == sec {
			sum += atomic.LoadInt64(&b.sum)
			count += atomic.LoadInt64(&b.cnt)
		}
	}
	return
}

func (s *paymentService) Purge(ctx context.Context) {
	for i := range s.buckets {
		atomic.StoreInt64(&s.buckets[i].ts, 0)
		atomic.StoreInt64(&s.buckets[i].sum, 0)
		atomic.StoreInt64(&s.buckets[i].cnt, 0)
	}
	if s.redis != nil {
		s.redis.Purge(ctx)
	}
}
