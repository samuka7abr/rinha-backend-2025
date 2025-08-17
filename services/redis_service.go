package services

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisMsg struct {
	sec    int64
	amount int64
}

type RedisService struct {
	c      *redis.Client
	ttl    time.Duration
	in     chan redisMsg
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func (s *RedisService) run(ctx context.Context) {
	defer s.wg.Done()

	const (
		flushEvery   = 20 * time.Millisecond
		maxQueueDrain = 4096                
	)

	t := time.NewTicker(flushEvery)
	defer t.Stop()

	sums := make(map[int64]int64)
	counts := make(map[int64]int64)

	flush := func() {
		if len(sums) == 0 {
			return
		}
		pipe := s.c.Pipeline()
		for sec, sum := range sums {
			key := "pay:sec:" + strconv.FormatInt(sec, 10)
			pipe.HIncrBy(ctx, key, "sum", sum)
			pipe.HIncrBy(ctx, key, "cnt", counts[sec])
			pipe.ExpireNX(ctx, key, s.ttl)
		}
		_, _ = pipe.Exec(ctx) 
		for k := range sums {
			delete(sums, k)
			delete(counts, k)
		}
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case <-t.C:
			flush()
		case msg := <-s.in:
			n := 0
			for {
				sums[msg.sec] += msg.amount
				counts[msg.sec]++
				n++
				if n >= maxQueueDrain {
					break
				}
				select {
				case msg = <-s.in:
				default:
					goto drained
				}
			}
		drained:
		}
	}
}

func NewRedisService(parent context.Context) (*RedisService, error) {
	addr := getenv("REDIS_ADDR", "redis:6379")
	ttlSec := getint("REDIS_TTL_SEC", 180)

	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		PoolSize:     64,
		MinIdleConns: 16,
		DialTimeout:  50 * time.Millisecond,
		ReadTimeout:  50 * time.Millisecond,
		WriteTimeout: 50 * time.Millisecond,
		PoolTimeout:  50 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(parent, 500*time.Millisecond)
	if err := rdb.Ping(ctx).Err(); err != nil {
		cancel()
		return nil, err
	}
	cancel()

	ctx2, cancel2 := context.WithCancel(parent)
	s := &RedisService{
		c:      rdb,
		ttl:    time.Duration(ttlSec) * time.Second,
		in:     make(chan redisMsg, 1<<16),
		cancel: cancel2,
	}
	s.wg.Add(1)
	go s.run(ctx2)
	return s, nil
}

func (s *RedisService) Enqueue(sec, amount int64) {
	select {
	case s.in <- redisMsg{sec: sec, amount: amount}:
	default:
	}
}

func (s *RedisService) BumpBucket(ctx context.Context, sec int64, amount int64) {
	s.Enqueue(sec, amount)
}

func (s *RedisService) Purge(ctx context.Context) {
	var cursor uint64
	for i := 0; i < 1000; i++ {
		keys, cur, err := s.c.Scan(ctx, cursor, "pay:sec:*", 1000).Result()
		if err != nil {
			break
		}
		if len(keys) > 0 {
			_ = s.c.Del(ctx, keys...).Err()
		}
		cursor = cur
		if cursor == 0 {
			break
		}
	}
}


func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func getint(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return d
}
